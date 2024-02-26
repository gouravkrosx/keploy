package cmd

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/TheZeroSlave/zapsentry"
	sentry "github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"
	"go.keploy.io/server/pkg/platform/fs"
	"go.keploy.io/server/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var Emoji = "\U0001F430" + " Keploy:"

var errFileNotFound = errors.New("fileNotFound")

type Root struct {
	logger *zap.Logger
	// subCommands holds a list of registered plugins.
	subCommands []Plugins
}

var debugMode bool
var enableTesting bool
var mode string

type colorConsoleEncoder struct {
	*zapcore.EncoderConfig
	zapcore.Encoder
}

func NewColorConsole(cfg zapcore.EncoderConfig) (enc zapcore.Encoder) {
	return colorConsoleEncoder{
		EncoderConfig: &cfg,
		// Using the default ConsoleEncoder can avoid rewriting interfaces such as ObjectEncoder
		Encoder: zapcore.NewConsoleEncoder(cfg),
	}
}

// EncodeEntry overrides ConsoleEncoder's EncodeEntry
func (c colorConsoleEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (buf *buffer.Buffer, err error) {
	buff, err := c.Encoder.EncodeEntry(ent, fields) // Utilize the existing implementation of zap
	if err != nil {
		return nil, err
	}

	bytesArr := bytes.Replace(buff.Bytes(), []byte("\\u001b"), []byte("\u001b"), -1)
	buff.Reset()
	buff.AppendString(string(bytesArr))
	return buff, err
}

// Clone overrides ConsoleEncoder's Clone
func (c colorConsoleEncoder) Clone() zapcore.Encoder {
	clone := c.Encoder.Clone()
	return colorConsoleEncoder{
		EncoderConfig: c.EncoderConfig,
		Encoder:       clone,
	}
}

func init() {
	_ = zap.RegisterEncoder("colorConsole", func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
		return NewColorConsole(config), nil
	})
}

func setupLogger() *zap.Logger {
	logCfg := zap.NewDevelopmentConfig()

	logCfg.Encoding = "colorConsole"

	// Customize the encoder config to put the emoji at the beginning.
	logCfg.EncoderConfig.EncodeTime = customTimeEncoder
	logCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logCfg.OutputPaths = []string{
		"stdout",
		"./keploy-logs.txt",
	}

	if debugMode {
		go func() {
			defer utils.HandlePanic()
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()

		logCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		logCfg.DisableStacktrace = false
	} else {
		logCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		logCfg.DisableStacktrace = true
		logCfg.EncoderConfig.EncodeCaller = nil
	}

	logger, err := logCfg.Build()
	if err != nil {
		log.Panic(Emoji, "failed to start the logger for the CLI")
		return nil
	}
	return logger
}

func modifyToSentryLogger(log *zap.Logger, client *sentry.Client) *zap.Logger {
	cfg := zapsentry.Configuration{
		Level:             zapcore.ErrorLevel, //when to send message to sentry
		EnableBreadcrumbs: true,               // enable sending breadcrumbs to Sentry
		BreadcrumbLevel:   zapcore.InfoLevel,  // at what level should we sent breadcrumbs to sentry
		Tags: map[string]string{
			"component": "system",
		},
	}
	core, err := zapsentry.NewCore(cfg, zapsentry.NewSentryClientFromClient(client))

	//in case of err it will return noop core. So we don't need to attach it to logger.
	if err != nil {
		log.Debug("failed to init zap", zap.Error(err))
		return log
	}

	log = zapsentry.AttachCoreToLogger(core, log)
	kernelVersion := ""
	if runtime.GOOS == "linux" {
		cmd := exec.Command("uname", "-r")
		kernelBytes, err := cmd.Output()
		if err != nil {
			log.Debug("failed to get kernel version", zap.Error(err))
		} else {
			kernelVersion = string(kernelBytes)
		}
	}
	arch := runtime.GOARCH
	installationID, err := fs.NewTeleFS(log).Get(false)
	if err != nil {
		log.Debug("failed to get installationID", zap.Error(err))
	}
	if installationID == "" {
		installationID, err = fs.NewTeleFS(log).Get(true)
		if err != nil {
			log.Debug("failed to get installationID for new user.", zap.Error(err))
		}
	}
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("Keploy Version", utils.KeployVersion)
		scope.SetTag("Linux Kernel Version", kernelVersion)
		scope.SetTag("Architecture", arch)
		scope.SetTag("Installation ID", installationID)
		// Add more context as needed
	})
	return log
}

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	emoji := "\U0001F430"
	if enableTesting {
		if mode == "record" {
			emoji += " Keploy(Record):"
		} else if mode == "test" {
			emoji += " Keploy(Test):"
		}
	} else {
		emoji += " Keploy:"
	}
	enc.AppendString(emoji + " " + t.Format(time.RFC3339) + " ")
}

func newRoot() *Root {
	return &Root{
		subCommands: []Plugins{},
	}
}

// Execute adds all child commands to the root command.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	newRoot().execute()
}

var rootCustomHelpTemplate = `{{.Short}}

Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Available Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableLocalFlags}}

Guided Commands:{{range .Commands}}{{if and (not .IsAvailableCommand) (not .Hidden)}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

Examples:
{{.Example}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

var rootExamples = `
  Record:
	keploy record -c "docker run -p 8080:8080 --name <containerName> --network keploy-network <applicationImage>" --containerName "<containerName>" --delay 1 --buildDelay 1m

  Test:
	keploy test --c "docker run -p 8080:8080 --name <containerName> --network keploy-network <applicationImage>" --delay 1 --buildDelay 1m

  Generate-Config:
	keploy generate-config -p "/path/to/localdir"
`

func checkForDebugFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--debug" {
			return true
		}
	}
	return false
}

func checkForTestBenchFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--enableTesting" {
			return true
		}
	}
	return false
}

func deleteLogs(logger *zap.Logger) {
	//Check if keploy-log.txt exists
	_, err := os.Stat("keploy-logs.txt")
	if os.IsNotExist(err) {
		return
	}
	//If it does, remove it.
	err = os.Remove("keploy-logs.txt")
	if err != nil {
		logger.Error("Error removing log file: %v\n", zap.String("error", err.Error()))
		return
	}
}

func (r *Root) execute() {
	// Root command
	var rootCmd = &cobra.Command{
		Use:     "keploy",
		Short:   "Keploy CLI",
		Example: rootExamples,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			subCmd, _, _ := cmd.Find(args)
			if subCmd != nil && subCmd.Name() != "keploy" {
				mode = subCmd.Name()
				println("And the mode is:", mode)
			}
		},
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpTemplate(rootCustomHelpTemplate)

	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Run in debug mode")

	// Manually parse flags to determine debug mode early
	debugMode = checkForDebugFlag(os.Args[1:])

	rootCmd.PersistentFlags().BoolVar(&enableTesting, "enableTesting", false, "Enable testing keploy with keploy")
	//for test bench manually parse flags to determine keploy mode & is test bench enabled.
	enableTesting = checkForTestBenchFlag(os.Args[1:])

	// Now that flags are parsed, set up the l722ogger
	r.logger = setupLogger()
	r.logger = modifyToSentryLogger(r.logger, sentry.CurrentHub().Client())
	defer deleteLogs(r.logger)
	r.subCommands = append(r.subCommands, NewCmdRecord(r.logger), NewCmdTest(r.logger), NewCmdServe(r.logger), NewCmdExample(r.logger), NewCmdMockRecord(r.logger), NewCmdMockTest(r.logger), NewCmdGenerateConfig(r.logger))

	// add the registered keploy plugins as subcommands to the rootCmd
	for _, sc := range r.subCommands {
		rootCmd.AddCommand(sc.GetCmd())
	}

	if err := rootCmd.Execute(); err != nil {
		r.logger.Error("failed to start the CLI.", zap.Any("error", err.Error()))
		os.Exit(1)
	}
}

// Plugins is an interface used to define plugins.
type Plugins interface {
	GetCmd() *cobra.Command
}

// RegisterPlugin registers a plugin by appending it to the list of subCommands.
func (r *Root) RegisterPlugin(p Plugins) {
	r.subCommands = append(r.subCommands, p)
}

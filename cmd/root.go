package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var Emoji = "\U0001F430" + " Keploy:"

type Root struct {
	logger *zap.Logger
	// subCommands holds a list of registered plugins.
	subCommands []Plugins
}

func newRoot() *Root {
	// logger init
	logCfg := zap.NewDevelopmentConfig()
	logCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, err := logCfg.Build()
	if err != nil {
		log.Panic(Emoji, "failed to start the logger for the CLI")
		return nil
	}

	return &Root{
		logger:      logger,
		subCommands: []Plugins{},
	}
}

// Execute adds all child commands to the root command.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	newRoot().execute()
}

// execute creates a root command for Cobra. The root cmd will be executed after attaching the subcommmands.
func (r *Root) execute() {
	// Root command
	var rootCmd = &cobra.Command{
		Use:   "keploy",
		Short: "Keploy CLI",
		// Run: func(cmd *cobra.Command, args []string) {

		// },
	}
	// rootCmd.Flags().IntP("pid", "", 0, "Please enter the process id on which your application is running.")

	r.subCommands = append(r.subCommands, NewCmdRecord(r.logger), NewCmdTest(r.logger))

	// add the registered keploy plugins as subcommands to the rootCmd
	for _, sc := range r.subCommands {
		rootCmd.AddCommand(sc.GetCmd())
	}

	if err := rootCmd.Execute(); err != nil {
		r.logger.Error(Emoji+"failed to start the CLI.", zap.Any("error", err.Error()))
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

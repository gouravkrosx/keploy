package serve

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"go.keploy.io/server/pkg"
	"go.keploy.io/server/pkg/hooks"
	"go.keploy.io/server/pkg/models"
	"go.keploy.io/server/pkg/platform/fs"
	"go.keploy.io/server/pkg/platform/telemetry"
	"go.keploy.io/server/pkg/platform/yaml"
	"go.keploy.io/server/pkg/proxy"
	"go.keploy.io/server/pkg/service/serve/graph"
	"go.keploy.io/server/pkg/service/test"
	"go.keploy.io/server/utils"
	"go.uber.org/zap"
)

var Emoji = "\U0001F430" + " Keploy:"

type server struct {
	logger *zap.Logger
	mutex  sync.Mutex
}

func NewServer(logger *zap.Logger) Server {
	return &server{
		logger: logger,
		mutex:  sync.Mutex{},
	}
}

const defaultPort = 6789

// Serve is called by the serve command and is used to run a graphql server, to run tests separately via apis.
func (s *server) Serve(path string, proxyPort uint32, testReportPath string, Delay uint64, pid, port uint32, lang string, passThroughPorts []uint, apiTimeout uint64, appCmd string, enableTele bool) {
	var ps *proxy.ProxySet

	if port == 0 {
		port = defaultPort
	}

	// Listen for the interrupt signal
	stopper := make(chan os.Signal, 1)
	signal.Notify(stopper, syscall.SIGINT, syscall.SIGTERM)

	models.SetMode(models.MODE_TEST)
	tester := test.NewTester(s.logger)
	testReportFS := yaml.NewTestReportFS(s.logger)
	teleFS := fs.NewTeleFS(s.logger)
	tele := telemetry.NewTelemetry(enableTele, false, teleFS, s.logger, "", nil)
	tele.Ping(false)
	ys := yaml.NewYamlStore("", "", "", "", s.logger, tele)
	routineId := pkg.GenerateRandomID()
	// Initiate the hooks
	loadedHooks, err := hooks.NewHook(ys, routineId, s.logger)
	if err != nil {
		s.logger.Error("error while creating hooks", zap.Error(err))
		return
	}

	// Recover from panic and gracfully shutdown
	defer loadedHooks.Recover(routineId)

	ctx := context.Background()

	// load the ebpf hooks into the kernel
	select {
	case <-stopper:
		return
	default:
		// load the ebpf hooks into the kernel
		if err := loadedHooks.LoadHooks("", "", pid, ctx, nil); err != nil {
			return
		}
	}

	//sending this graphql server port to be filterd in the eBPF program
	if err := loadedHooks.SendKeployServerPort(port); err != nil {
		return
	}

	select {
	case <-stopper:
		loadedHooks.Stop(true)
		return
	default:
		// start the proxy
		ps = proxy.BootProxy(s.logger, proxy.Option{Port: proxyPort}, "", "", pid, lang, passThroughPorts, loadedHooks, ctx, 0)

	}

	// proxy update its state in the ProxyPorts map
	// Sending Proxy Ip & Port to the ebpf program
	if err := loadedHooks.SendProxyInfo(ps.IP4, ps.Port, ps.IP6); err != nil {
		return
	}

	s.logger.Info("Adding default jacoco agent port to passthrough", zap.Uint("Port", 36320))
	passThroughPorts = append(passThroughPorts, 36320)
	// filter the required destination ports
	if err := loadedHooks.SendPassThroughPorts(passThroughPorts); err != nil {
		return
	}

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{
			Tester:         tester,
			TestReportFS:   testReportFS,
			YS:             ys,
			LoadedHooks:    loadedHooks,
			Logger:         s.logger,
			Path:           path,
			TestReportPath: testReportPath,
			Delay:          Delay,
			AppPid:         pid,
			ApiTimeout:     apiTimeout,
			ServeTest:      len(appCmd) != 0,
		},
	}))

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)

	// Create a new http.Server instance
	httpSrv := &http.Server{
		Addr:    ":" + strconv.Itoa(int(port)),
		Handler: nil, // Use the default http.DefaultServeMux
	}

	// Create a shutdown channel

	// Start your server in a goroutine
	go func() {
		// Recover from panic and gracefully shutdown
		defer loadedHooks.Recover(pkg.GenerateRandomID())
		defer utils.HandlePanic()
		log.Printf(Emoji+"connect to http://localhost:%d/ for GraphQL playground", port)
		if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf(Emoji+"listen: %s\n", err)
		}
		s.logger.Debug("graphql server stopped")
	}()

	defer s.stopGraphqlServer(httpSrv)

	abortStopHooksInterrupt := make(chan bool) // channel to stop closing of keploy via interrupt
	exitCmd := make(chan bool)                 // channel to exit this command

	// Block until we receive one
	abortStopHooksForcefully := false
	select {
	case <-stopper:
		loadedHooks.Stop(true)
		ps.StopProxyServer()
		return
	default:
		go func() {
			if err := loadedHooks.LaunchUserApplication(appCmd, "", "", Delay, 30*time.Second, true, false); err != nil {
				switch err {
				case hooks.ErrInterrupted:
					s.logger.Info("keploy terminated user application")
					return
				case hooks.ErrFailedUnitTest:
					s.logger.Debug("unit tests failed hence stopping keploy")
				case hooks.ErrUnExpected:
					s.logger.Debug("unit tests ran successfully hence stopping keploy")
				default:
					s.logger.Error("unknown error recieved from application", zap.Error(err))
				}
			}
			if !abortStopHooksForcefully {
				abortStopHooksInterrupt <- true
				// stop listening for the eBPF events
				loadedHooks.Stop(true)
				ps.StopProxyServer()
				exitCmd <- true
				//stop listening for proxy server
			} else {
				return
			}

		}()
	}
	select {
	case <-stopper:
		abortStopHooksForcefully = true
		loadedHooks.Stop(false)
		ps.StopProxyServer()
		return
	case <-abortStopHooksInterrupt:
		//telemetry event can be added here
	}
	<-exitCmd
}

// Gracefully shut down the HTTP server with a timeout
func (s *server) stopGraphqlServer(httpSrv *http.Server) {
	shutdown := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		s.logger.Error("Graphql server shutdown failed", zap.Error(err))
	}
	// If you have other goroutines that should listen for this, you can use this channel to notify them.
	close(shutdown)
}

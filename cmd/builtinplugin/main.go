package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/cmd/builtinplugin/argconfig"
	"meoe.io/poly-csi/cmd/builtinplugin/connectorpluginconfig"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/blockcon/cntrublk/supervisor"
	"meoe.io/poly-csi/pkg/node/blockcon/cntrublk/ublkfactory"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrhttpjsonplugin"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
	"meoe.io/poly-csi/pkg/transport/localexec"

	slogctx "github.com/veqryn/slog-context"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// NotifyContext returns stop() instead of cancel,
	// but in the main stop is not useful
	ctx, _ = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	argConfig := argconfig.Parse()

	logLevel := new(slog.LevelVar)
	logLevel.Set(slog.LevelInfo)
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(logHandler)

	logger = logger.With("host", argConfig.Hostname)
	ctx = slogctx.NewCtx(ctx, logger)

	logger.Info("using arg config", "config", argConfig)

	// set log level after the 'using config' log entry, so that it is always present
	logLevel.Set(argConfig.LogLevel)

	configContent, err := os.ReadFile(argConfig.ConfigPath)
	if err != nil {
		panic(err.Error())
	}

	var config connectorpluginconfig.Config
	err = yaml.Unmarshal(configContent, &config)
	if err != nil {
		panic("failed to parse config: " + err.Error())
	}

	tr := localexec.New(localexec.Config{})
	// TODO use this more
	logsLogger := config.Logs.MakeInstance()
	logsLogger.Info("parsed config", "config", config)
	ctx = logs.NewCtx(ctx, logsLogger)

	serverDoneCh := make(chan struct{})

	var handler http.Handler

	switch argConfig.PluginType {
	case "connector":
		cons := connectorfactory.ParseList(logsLogger, tr, config.DevFolder, config.Connectors, connectorfactory.GetByType)
		dispatcher := cntrhttpjsonplugin.NewDispatcherServer(tr, config.DevFolder, cons)
		handler = cntrhttpjsonplugin.NewHttpServer(ctx, dispatcher, config.Logs)

	case "ublk":
		err = os.MkdirAll(config.Supervisor.ConfigDir, 0777)
		if err != nil {
			panic(err)
		}
		supervisor := supervisor.New(ctx, tr, config.Supervisor.ConfigDir, config.Supervisor.MinId, config.Supervisor.MaxId)
		logsLogger.Info("running supervisor.StartupRestore")
		err := supervisor.StartupRestore()
		if err != nil {
			panic(err)
		}
		logsLogger.Info("finished supervisor.StartupRestore")

		cons := connectorfactory.ParseList(logsLogger, tr, config.DevFolder, config.Connectors, func(mtype string) (connectors.InflateFunc, error) {
			inflate, err := ublkfactory.GetByType(mtype)
			if err != nil {
				return nil, err
			}
			return func(conf connectors.CommonConfig) (connectors.Manager, error) {
				return inflate(supervisor, conf), nil
			}, nil
		})

		dispatcher := cntrhttpjsonplugin.NewDispatcherServer(tr, config.DevFolder, cons)
		handler = cntrhttpjsonplugin.NewHttpServer(ctx, dispatcher, config.Logs)
	default:
		panic("unknown plugin type: " + argConfig.PluginType)
	}

	// if previous run of the plugin failed without cleanup (sigkill, OOM, etc),
	// dead socket file will prevent us from binding
	if err := os.RemoveAll(config.Socket); err != nil {
		panic(err)
	}
	serverInstance := NewServer(ctx, config.Socket, handler)

	serverInstance.StartServe(ctx, time.Second, func(err error) {
		if err != nil {
			logsLogger.Error("server finished with error", "error", err.Error())
		} else {
			logsLogger.Info("server finished without error")
		}
		close(serverDoneCh)
	})

	select {
	case <-serverDoneCh:
		cancel()
		logsLogger.Info("server exited")
	case <-ctx.Done():
		logsLogger.Info("context cancelled")
	}
	<-serverDoneCh
	logsLogger.Info("shutdown finished")
}

type Server struct {
	socket  string
	callCtx context.Context
	hs      http.Handler
}

func NewServer(callCtx context.Context, socket string, hs http.Handler) *Server {
	return &Server{
		callCtx: callCtx,
		socket:  socket,
		hs:      hs,
	}
}

func (s *Server) StartServe(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error)) {
	mux := http.NewServeMux()
	mux.Handle(cntrhttpjsonplugin.ConnectorHttpPath, s.hs)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	serveDoneCh := make(chan struct{})

	go func() {
		defer close(serveDoneCh)
		unixListener, err := net.Listen("unix", s.socket)
		if err != nil {
			doneCallback(err)
			return
		}
		err = srv.Serve(unixListener)
		doneCallback(err)
	}()
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()
			slogctx.FromCtx(ctx).Info("context done, running server shutdown")
			_ = srv.Shutdown(shutdownCtx)
		case <-serveDoneCh:
		}
	}()
}

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

	"meoe.io/poly-csi/cmd/builtinplugin/argconfig"
	"meoe.io/poly-csi/cmd/builtinplugin/connectorpluginconfig"
	"meoe.io/poly-csi/pkg/node/connectors/cntrhttpjsonplugin"
	"meoe.io/poly-csi/pkg/transport/localexec"

	slogctx "github.com/veqryn/slog-context"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// NotifyContext returns stop() instead of cancel,
	// but in the main stop is not useful
	ctx, _ = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	appConfig := argconfig.Parse()

	logLevel := new(slog.LevelVar)
	logLevel.Set(slog.LevelInfo)
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(logHandler)

	logger = logger.With("host", appConfig.Hostname)
	ctx = slogctx.NewCtx(ctx, logger)

	logger.Info("using config", "config", appConfig)

	// set log level after the 'using config' log entry, so that it is always present
	logLevel.Set(appConfig.LogLevel)

	tr := localexec.New(localexec.Config{})

	serverDoneCh := make(chan struct{})
	switch appConfig.PluginType {
	case "connector":
		configContent, err := os.ReadFile(appConfig.ConfigPath)
		if err != nil {
			panic(err.Error())
		}
		conf, err := connectorpluginconfig.ParseConfig(ctx, tr, configContent)
		if err != nil {
			panic(err.Error())
		}

		if !conf.Connector.Valid {
			panic("connector config is not valid")
		}

		serverInstance := NewServer(ctx, conf.Config.Socket, conf.Srv)

		serverInstance.StartServe(ctx, time.Second, func(err error) {
			if err != nil {
				logger.Error("server finished with error", "error", err.Error())
			} else {
				logger.Info("server finished without error")
			}
			close(serverDoneCh)
		})
	}

	select {
	case <-serverDoneCh:
		cancel()
		logger.Info("server exited")
	case <-ctx.Done():
		logger.Info("context cancelled")
	}
	<-serverDoneCh
	logger.Info("shutdown finished")
}

type Server struct {
	socket  string
	callCtx context.Context
	hs      *cntrhttpjsonplugin.HttpServer
}

func NewServer(callCtx context.Context, socket string, hs *cntrhttpjsonplugin.HttpServer) *Server {
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

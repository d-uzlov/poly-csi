package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/cmd/builtinplugin/config"
	"meoe.io/poly-csi/cmd/builtinplugin/connectorpluginconfig"
	"meoe.io/poly-csi/pkg/node/connectors/cntrhttpjsonplugin"
	"meoe.io/poly-csi/pkg/transport/localexec"

	slogctx "github.com/veqryn/slog-context"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	appConfig := config.ParseConfig()

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

		serverByteConfig, err := yaml.Marshal(conf.Config.ServerConfig)
		if err != nil {
			panic(fmt.Errorf("marshal server config: %w", err))
		}

		var cfg ServerConfig
		if err := yaml.Unmarshal(serverByteConfig, &cfg); err != nil {
			panic(fmt.Errorf("parse %s config: %w", conf.Config.ServerType, err))
		}

		serverInstance := NewServer(ctx, cfg, conf.Srv)

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
	config  ServerConfig
	callCtx context.Context
	hs      *cntrhttpjsonplugin.HttpServer
}

type ServerConfig struct {
	Socket string `yaml:"socket"`
}

func NewServer(callCtx context.Context, config ServerConfig, hs *cntrhttpjsonplugin.HttpServer) *Server {
	return &Server{
		callCtx: callCtx,
		config:  config,
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
		unixListener, err := net.Listen("unix", s.config.Socket)
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

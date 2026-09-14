package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"meoe.io/poly-csi/cmd/builtinplugin/config"
	"meoe.io/poly-csi/cmd/builtinplugin/connectorpluginconfig"
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
		conf, err := connectorpluginconfig.ParseConfig(ctx, configContent)
		if err != nil {
			panic(err.Error())
		}

		if !conf.Connector.Valid {
			panic("connector config is not valid")
		}

		instance := conf.Connector.Inflate()
		if instance == nil {
			panic("connector inflate failed")
		}
		// TODO add support for presets
		err = instance.Configure(ctx, tr, map[string]string{})
		if err != nil {
			panic(err)
		}
		serverInstance := conf.InflateServer(ctx, instance)

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

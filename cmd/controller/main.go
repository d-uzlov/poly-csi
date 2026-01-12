package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/cmd/controller/config"
	"meoe.io/poly-csi/pkg/controller"
	"meoe.io/poly-csi/pkg/controller/controllerserver"
	"meoe.io/poly-csi/pkg/controller/dispatcher"
	"meoe.io/poly-csi/pkg/utils/configwatcher"
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

	configContent, err := os.ReadFile(appConfig.ConfigPath)
	if err != nil {
		panic(err.Error())
	}
	dispatcherConfig, err := dispatcher.ParseDispatcherConfig(ctx, configContent)
	if err != nil {
		panic(err.Error())
	}

	d := dispatcher.NewDispatcher(ctx)
	d.ReloadConfig(ctx, dispatcherConfig)
	// TODO it's bad to use program context in cleanup, it can be expired by the time cleanup is called
	defer d.Cleanup(ctx)

	go configwatcher.WatchConfigChanges(ctx, appConfig.ConfigPath, func() {
		logger.Info("reading new config file")
		newContent, err := os.ReadFile(appConfig.ConfigPath)
		if err != nil {
			logger.Error("reading config file failed", "error", err)
			return
		}
		newConfig, err := dispatcher.ParseDispatcherConfig(ctx, newContent)
		if err != nil {
			logger.Error("parsing config file failed", "error", err)
			return
		}
		d.ReloadConfig(ctx, newConfig)
	})

	con, err := controller.CreateController(logger, d, appConfig.CsiName, config.GetVersion())
	if err != nil {
		logger.Error("could not create controller", "error", err)
		panic(err)
	}
	server := controllerserver.NewControllerServer(logger)
	doneCh := make(chan struct{})

	err = server.ListenAndServe(ctx, appConfig.ListenAddress, con, func() {
		close(doneCh)
	})
	if err != nil {
		panic(err.Error())
	}
	select {
	case <-doneCh:
		cancel()
		logger.Info("server exited")
	case <-ctx.Done():
		logger.Info("context cancelled")
	}
	<-doneCh
	logger.Info("shutdown finished")
}

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/cmd/controller/argconfig"
	"meoe.io/poly-csi/pkg/controller"
	"meoe.io/poly-csi/pkg/controller/controllerserver"
	"meoe.io/poly-csi/pkg/controller/dispatcher"
	"meoe.io/poly-csi/pkg/utils/configwatcher"
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
	dispatcherConfig, err := dispatcher.ParseDispatcherConfig(ctx, configContent)
	if err != nil {
		panic(err.Error())
	}

	d := dispatcher.NewDispatcher(ctx)
	d.ReloadConfig(ctx, dispatcherConfig)
	defer d.Cleanup()

	go configwatcher.WatchConfigChanges(ctx, argConfig.ConfigPath, func() {
		logger.Info("reading new config file")
		newContent, err := os.ReadFile(argConfig.ConfigPath)
		if err != nil {
			logger.Error("reading config file failed", "error", err.Error())
			return
		}
		newConfig, err := dispatcher.ParseDispatcherConfig(ctx, newContent)
		if err != nil {
			logger.Error("parsing config file failed", "error", err.Error())
			return
		}
		d.ReloadConfig(ctx, newConfig)
	})

	con, err := controller.CreateController(logger, d, argConfig.CsiName, argconfig.GetVersion())
	if err != nil {
		panic(err)
	}
	server := controllerserver.NewControllerServer(con)
	doneCh := make(chan struct{})

	if strings.HasPrefix(argConfig.ListenAddress, "unix") {
		// if previous run of the plugin failed without cleanup (sigkill, OOM, etc),
		// dead socket file will prevent us from binding
		if err := os.RemoveAll(argConfig.ListenAddress); err != nil {
			panic(err)
		}
	}
	err = server.ListenAndServe(ctx, argConfig.ListenAddress, func() {
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

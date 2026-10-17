package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"meoe.io/poly-csi/cmd/node/argconfig"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/nodeconfig"
	"meoe.io/poly-csi/pkg/node/nodedispatcher"
	"meoe.io/poly-csi/pkg/node/nodeserver"
	"meoe.io/poly-csi/pkg/utils/configwatcher"

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

	// TODO use this more
	logsConfig := logs.Config{
		LogLevel: "info",
	}
	logsLogger := logsConfig.MakeInstance()
	ctx = logs.NewCtx(ctx, logsLogger)

	configContent, err := os.ReadFile(argConfig.ConfigPath)
	if err != nil {
		panic(err.Error())
	}
	initialNodeConfig, err := nodeconfig.ParseConfig(ctx, configContent)
	if err != nil {
		panic(err.Error())
	}

	nodeInstance, err := nodedispatcher.New(logger, argConfig.CsiName, argconfig.GetVersion(), argConfig.CsiNodeId)
	if err != nil {
		logger.Error("could not create node controller", "error", err.Error())
		panic(err)
	}

	err = nodeInstance.ReloadConfig(ctx, initialNodeConfig)
	if err != nil {
		panic(err.Error())
	}

	go configwatcher.WatchConfigChanges(ctx, argConfig.ConfigPath, func() {
		logger.Info("config reload request")
		configContent, err := os.ReadFile(argConfig.ConfigPath)
		if err != nil {
			panic(err.Error())
		}
		nodeConfig, err := nodeconfig.ParseConfig(ctx, configContent)
		if err != nil {
			panic(err.Error())
		}
		err = nodeInstance.ReloadConfig(ctx, nodeConfig)
		if err != nil {
			logger.Error("config reload failed", "error", err.Error())
			return
		}
		logger.Info("config reload success")
	})
	server := nodeserver.NewNodeServer(ctx, nodeInstance)
	serverDoneCh := make(chan struct{})

	if strings.HasPrefix(argConfig.ListenAddress, "unix") {
		// if previous run of the plugin failed without cleanup (sigkill, OOM, etc),
		// dead socket file will prevent us from binding
		if err := os.RemoveAll(argConfig.ListenAddress); err != nil {
			panic(err)
		}
	}
	err = server.GoListenAndServe(ctx, argConfig.ListenAddress, func() { close(serverDoneCh) })
	if err != nil {
		panic(err.Error())
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

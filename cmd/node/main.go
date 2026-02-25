package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"meoe.io/poly-csi/cmd/node/config"
	"meoe.io/poly-csi/pkg/node/nodedispatcher"
	"meoe.io/poly-csi/pkg/node/nodeserver"
	"meoe.io/poly-csi/pkg/utils/configwatcher"

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

	configContent, err := os.ReadFile(appConfig.ConfigPath)
	if err != nil {
		panic(err.Error())
	}
	_, err = nodedispatcher.ParseConfig(ctx, configContent)
	if err != nil {
		panic(err.Error())
	}

	nodeInstance, err := nodedispatcher.CreateNode(logger, appConfig.CsiName, config.GetVersion(), appConfig.CsiNodeId)
	if err != nil {
		logger.Error("could not create node controller", "error", err)
		panic(err)
	}
	go configwatcher.WatchConfigChanges(ctx, appConfig.ConfigPath, func() {
		logger.Info("config reload request")
		configContent, err := os.ReadFile(appConfig.ConfigPath)
		if err != nil {
			panic(err.Error())
		}
		nodeConfig, err := nodedispatcher.ParseConfig(ctx, configContent)
		if err != nil {
			panic(err.Error())
		}
		err = nodeInstance.ReloadConfig(ctx, *nodeConfig)
		if err != nil {
			logger.Error("config reload failed", "error", err)
			return
		}
		logger.Info("config reload success")
	})
	server := nodeserver.NewNodeServer(ctx, logger)
	serverDoneCh := make(chan struct{})

	err = server.GoListenAndServe(ctx, appConfig.ListenAddress, nodeInstance, func() { close(serverDoneCh) })
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

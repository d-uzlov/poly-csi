package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"meoe.io/poly-csi/cmd/noderegistry/argconfig"
	"meoe.io/poly-csi/pkg/noderegistry/noderegsrv"

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

	doneCh := make(chan struct{})

	reg := noderegsrv.NewNodeRegistry(logger)
	reg.StartNodeRegistry(ctx, time.Second, func(err error) {
		if err != nil {
			logger.Error("error on shutdown", "error", err.Error())
		}
		close(doneCh)
	})

	select {
	case <-ctx.Done():
		logger.Info("context cancelled")
	case <-doneCh:
		cancel()
		logger.Info("server exited")
	}
	<-doneCh
	logger.Info("shutdown finished")
}

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/cmd/controller/argconfig"
	"meoe.io/poly-csi/pkg/controller/dispatcher"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// NotifyContext returns stop() instead of cancel,
	// but in the main stop is not useful
	ctx, _ = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	appConfig := argconfig.Parse()
	_ = appConfig

	logLevel := new(slog.LevelVar)
	logLevel.Set(slog.LevelInfo)
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(logHandler)
	_ = logger

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

	el, wl := dispatcherConfig.Validate()
	for _, iss := range el {
		logger.Error(iss)
	}
	for _, iss := range wl {
		logger.Warn(iss)
	}
	if len(el) > 0 {
		os.Exit(1)
	}
}

package config

import (
	"flag"
	"log/slog"
	"os"

	"github.com/kouhin/envflag"
)

type AppConfig struct {
	LogLevel      slog.Level
}

const (
	CgroupFromNone    = "none"
	CgroupFromPid     = "pid"
	CgroupFromSpec    = "spec"
	CgroupFromSpecPid = "spec-pid"
)

func ParseConfig() *AppConfig {
	result := &AppConfig{}

	var versionFlag bool
	flag.BoolVar(&versionFlag, "version", false, "Print version info")

	var logLevel string
	flag.StringVar(&logLevel, "log-level", "info", "One of: error, warn, info, debug")

	if err := envflag.Parse(); err != nil {
		panic(err)
	}

	if versionFlag {
		printVersion()
		os.Exit(0)
	}

	switch logLevel {
	case "error":
		result.LogLevel = slog.LevelError
	case "warn":
		result.LogLevel = slog.LevelWarn
	case "info":
		result.LogLevel = slog.LevelInfo
	case "debug":
		result.LogLevel = slog.LevelDebug
	default:
		panic("unknown log level: " + logLevel)
	}

	return result
}

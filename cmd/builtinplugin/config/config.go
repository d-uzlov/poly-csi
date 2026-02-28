package config

import (
	"flag"
	"log/slog"
	"os"

	"github.com/kouhin/envflag"
)

type AppConfig struct {
	LogLevel   slog.Level
	Hostname   string
	ConfigPath string
	PluginType string
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

	flag.StringVar(&result.ConfigPath, "node-config", "./config-iscsi.yaml", "Path to application config")
	flag.StringVar(&result.PluginType, "plugin-type", "connector", "allowed values: connector, multipath")

	flag.StringVar(&result.Hostname, "hostname", "", "Hostname to use in logs, if it needs to be different from OS-provided value")
	var logLevel string
	flag.StringVar(&logLevel, "log-level", "info", "One of: error, warn, info, debug")

	if err := envflag.Parse(); err != nil {
		panic(err)
	}

	if versionFlag {
		printVersion()
		os.Exit(0)
	}

	if result.Hostname == "" {
		result.Hostname, _ = os.Hostname()
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

package argconfig

import (
	"flag"
	"log/slog"
	"os"

	"github.com/kouhin/envflag"
)

type ArgConfig struct {
	LogLevel      slog.Level
	CsiName       string
	Hostname      string
	ListenAddress string
	ConfigPath    string
}

const (
	CgroupFromNone    = "none"
	CgroupFromPid     = "pid"
	CgroupFromSpec    = "spec"
	CgroupFromSpecPid = "spec-pid"
)

func Parse() ArgConfig {
	result := ArgConfig{}

	var versionFlag bool
	flag.BoolVar(&versionFlag, "version", false, "Print version info")

	flag.StringVar(&result.CsiName, "csi-name", "poly-csi", "Name to be reported in csi plugin info")
	flag.StringVar(&result.ListenAddress, "listen-address", "tcp://[::]:4350", "Address to listen on. Can be 'tcp://ip:port' or 'unis://unix-path'")
	flag.StringVar(&result.ConfigPath, "config", "./config.yaml", "Path to application config")

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

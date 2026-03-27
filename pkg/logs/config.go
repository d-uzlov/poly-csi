package logs

import (
	"log/slog"
	"maps"
	"os"
	"strings"
)

type Config struct {
	LogLevel        string            `yaml:"logLevel"`
	EchoArgs        bool              `yaml:"echoArgs"`
	EchoCommands    bool              `yaml:"echoCommands"`
	ConfirmMicroOps bool              `yaml:"confirmMicroOps"`
	SkipOnSuccess   bool              `yaml:"skipOnSuccess"`
	ExtraFields     map[string]string `yaml:"extraFields"`
}

func (c Config) getLogLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "err", "error":
		return slog.LevelError
	case "none":
		return 1000
	default:
		return slog.LevelInfo
	}
}

func (c Config) Combine(another Config) Config {
	result := c
	if another.LogLevel != "" {
		result.LogLevel = another.LogLevel
	}

	if another.EchoArgs {
		result.EchoArgs = another.EchoArgs
	}
	if another.EchoCommands {
		result.EchoCommands = another.EchoCommands
	}
	if another.ConfirmMicroOps {
		result.ConfirmMicroOps = another.ConfirmMicroOps
	}
	if another.SkipOnSuccess {
		result.SkipOnSuccess = another.SkipOnSuccess
	}

	if result.ExtraFields == nil {
		result.ExtraFields = map[string]string{}
	} else {
		result.ExtraFields = maps.Clone(c.ExtraFields)
	}
	maps.Copy(result.ExtraFields, another.ExtraFields)

	return result
}

func (c Config) WithFields(fields ...string) Config {
	result := c
	if result.ExtraFields == nil {
		result.ExtraFields = map[string]string{}
	} else {
		result.ExtraFields = maps.Clone(c.ExtraFields)
	}
	for i := 0; i < len(fields)/2; i++ {
		result.ExtraFields[fields[i*2]] = fields[i*2+1]
	}
	return result
}

func (c Config) MakeInstance() *Instance {
	return c.MakeInstanceWithCopy(false)
}

func (c Config) MakeInstanceWithCopy(enableCopy bool) *Instance {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: c.getLogLevel(),
	})
	fields := []any{}
	for k, v := range c.ExtraFields {
		fields = append(fields, k, v)
	}
	logger := slog.New(logHandler).With(fields...)
	return &Instance{
		config:   c,
		logger:   logger,
		keepCopy: enableCopy,
	}
}

package connectorpluginconfig

import (
	"bytes"
	"context"
	"fmt"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/cmd/builtinplugin/connectorpluginserver"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
)

type ConnectorPlugin interface {
	StartServe(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error))
}

type Config struct {
	ServerType   string                        `yaml:"serverType"`
	ServerConfig map[string]any                `yaml:"serverConfig"`
	Connector    *connectorfactory.MountConfig `yaml:"connector"`
	DevFolder    string                        `yaml:"devFolder"`
	Logs         logs.Config                   `yaml:"logs"`
}

type Inflate func(ctx context.Context, connector connectors.Connector) ConnectorPlugin

type parsedConfig struct {
	InflateServer Inflate
	Connector     *connectorfactory.ParsedConnector
}

func ParseConfig(ctx context.Context, raw []byte) (*parsedConfig, error) {
	logger := slogctx.FromCtx(ctx)

	dec := yaml.NewDecoder(bytes.NewBuffer(raw))
	dec.KnownFields(true)

	var config Config
	err := dec.Decode(&config)
	// err := yaml.UnmarshalStrict(raw, &result)
	if err != nil {
		logger.Error("config: initial parsing failed", "error", err.Error())
		return nil, err
	}

	serverByteConfig, err := yaml.Marshal(config.ServerConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal server config: %w", err)
	}

	var cfg connectorpluginserver.ServerConfig
	if err := yaml.Unmarshal(serverByteConfig, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s config: %w", config.ServerType, err)
	}
	inflateServer := func(ctx context.Context, connector connectors.Connector) ConnectorPlugin {
		return connectorpluginserver.NewServer(ctx, cfg, connector, config.DevFolder, config.Logs)
	}

	pm := connectorfactory.ParseConnector(ctx, config.Connector, config.DevFolder)
	if pm == nil {
		return nil, fmt.Errorf("connectorfactory.ParseMount failed")
	}

	// el, wl := parsed.Validate()
	// for _, iss := range el {
	// 	logger.Error(fmt.Sprintf("servers[i=%d/id=%s].%s", i, parsed.conf.Id, iss))
	// }
	// for _, iss := range wl {
	// 	logger.Warn(fmt.Sprintf("servers[i=%d/id=%s].%s", i, parsed.conf.Id, iss))
	// }
	// if len(el) > 0 {
	// 	parsed.valid = false
	// }

	if pm.Valid {
		logger.Info(fmt.Sprintf("successfully parsed connector '%s'", pm.Conf.Id))
	} else {
		logger.Error(fmt.Sprintf("parsing connector '%s' failed", pm.Conf.Id))
	}

	return &parsedConfig{
		Connector:     pm,
		InflateServer: inflateServer,
	}, nil
}

package connectorpluginconfig

import (
	"bytes"
	"context"
	"fmt"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
	"meoe.io/poly-csi/pkg/node/mounts/mountplugins/mountpluginfactory"
)

type Config struct {
	ServerType   string                        `yaml:"serverType"`
	ServerConfig map[string]any                `yaml:"serverConfig"`
	Connector    *connectorfactory.MountConfig `yaml:"connector"`
	DevFolder    string                        `yaml:"devFolder"`
}

type parsedConfig struct {
	InflateServer mountpluginfactory.Inflate
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
		logger.Error("config: initial parsing failed", "error", err)
		return nil, err
	}

	serverByteConfig, err := yaml.Marshal(config.ServerConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal server config: %w", err)
	}

	inflateServer, err := mountpluginfactory.PluginServerFromType(config.ServerType, serverByteConfig, config.DevFolder)
	if err != nil {
		return nil, fmt.Errorf("mountpluginfactory.PluginServerFromType: %w", err)
	}

	pm := connectorfactory.ParseMount(ctx, config.Connector, config.DevFolder)
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

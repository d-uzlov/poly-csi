package mountpluginconfig

import (
	"bytes"
	"context"
	"fmt"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/mounts/mounterfactory"
	"meoe.io/poly-csi/pkg/node/mounts/mountplugins/mountpluginfactory"
)

type Config struct {
	ServerType   string                      `yaml:"serverType"`
	ServerConfig map[string]any              `yaml:"serverConfig"`
	Mounter      *mounterfactory.MountConfig `yaml:"mounter"`
}

type parsedConfig struct {
	InflateServer mountpluginfactory.Inflate
	Mounter       *mounterfactory.ParsedMount
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

	inflateServer, err := mountpluginfactory.PluginServerFromType(config.ServerType, serverByteConfig)
	if err != nil {
		return nil, fmt.Errorf("mountpluginfactory.PluginServerFromType: %w", err)
	}

	pm := mounterfactory.ParseMount(ctx, config.Mounter)
	if pm == nil {
		return nil, fmt.Errorf("mounterfactory.ParseMount failed")
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
		logger.Info(fmt.Sprintf("successfully parsed mounter '%s'", pm.Conf.Id))
	} else {
		logger.Error(fmt.Sprintf("parsing mounter '%s' failed", pm.Conf.Id))
	}

	return &parsedConfig{
		Mounter:       pm,
		InflateServer: inflateServer,
	}, nil
}

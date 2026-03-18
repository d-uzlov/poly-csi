package mppluginconfig

import (
	"bytes"
	"context"
	"fmt"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/node/multipath/mpplugin/mppluginfactory"
)

type Config struct {
	ServerType   string                     `yaml:"serverType"`
	ServerConfig map[string]any             `yaml:"serverConfig"`
	Multipath    *mpfactory.MultipathConfig `yaml:"multipath"`
	DevFolder    string                     `yaml:"devFolder"`
}

type parsedConfig struct {
	InflateServer mppluginfactory.Inflate
	Multipath     *mpfactory.ParsedMultipath
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

	inflateServer, err := mppluginfactory.PluginServerFromType(config.ServerType, serverByteConfig, config.DevFolder)
	if err != nil {
		return nil, fmt.Errorf("mountpluginfactory.PluginServerFromType: %w", err)
	}

	pm := mpfactory.ParseMultipath(ctx, config.Multipath, config.DevFolder)
	if pm == nil {
		return nil, fmt.Errorf("mpfactory.ParseMount failed")
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
		logger.Info(fmt.Sprintf("successfully parsed multipath '%s'", pm.Conf.Id))
	} else {
		logger.Error(fmt.Sprintf("parsing multipath '%s' failed", pm.Conf.Id))
	}

	return &parsedConfig{
		Multipath:     pm,
		InflateServer: inflateServer,
	}, nil
}

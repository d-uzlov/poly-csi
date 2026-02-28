package mppluginfactory

import (
	"context"
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/node/multipath/mpplugin"
	"meoe.io/poly-csi/pkg/node/multipath/mpplugin/mphttp"
)

type Inflate func(ctx context.Context, mp multipath.Multipath) mpplugin.MounterPlugin

func PluginServerFromType(serverType string, byteConfig []byte, devFolder string) (Inflate, error) {
	switch serverType {
	case "http-json":
		var cfg mphttp.ServerConfig
		if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s config: %w", serverType, err)
		}
		return func(ctx context.Context, mp multipath.Multipath) mpplugin.MounterPlugin {
			return mphttp.NewServer(ctx, cfg, mp, devFolder)
		}, nil
	default:
		return nil, fmt.Errorf("unknown server type: %s", serverType)
	}
}

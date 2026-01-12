package mountpluginfactory

import (
	"context"
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mountplugins"
	"meoe.io/poly-csi/pkg/node/mounts/mountplugins/mounthttp"
)

type Inflate func(ctx context.Context, mounter mounts.NodeMounter) mountplugins.MounterPlugin

func PluginServerFromType(serverType string, byteConfig []byte) (Inflate, error) {
	switch serverType {
	case "http-json":
		var cfg mounthttp.ServerConfig
		if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s config: %w", serverType, err)
		}
		return func(ctx context.Context, mounter mounts.NodeMounter) mountplugins.MounterPlugin {
			return mounthttp.NewServer(ctx, cfg, mounter)
		}, nil
	default:
		return nil, fmt.Errorf("unknown server type: %s", serverType)
	}
}

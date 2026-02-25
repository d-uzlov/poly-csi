package mounterfactory

import (
	"context"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/mounts"
)

type MountConfig struct {
	Id      string         `yaml:"id"`
	Type    string         `yaml:"type"`
	Subtype string         `yaml:"subtype"`
	Config  map[string]any `yaml:"config"`
}

type ParsedMount struct {
	Valid   bool
	Inflate func() mounts.NodeMounter
	Conf    *MountConfig
}

func ParseMount(ctx context.Context, c *MountConfig) *ParsedMount {
	logger := slogctx.FromCtx(ctx)

	if c.Type == "" {
		logger.Error("mount_config: missing type", "id", c.Id)
		return &ParsedMount{
			Valid: false,
			Conf:  c,
		}
	}

	byteConfig, err := yaml.Marshal(c.Config)
	if err != nil {
		logger.Error("mount_config: marshal config", "id", c.Id, "error", err)
		return &ParsedMount{
			Valid: false,
			Conf:  c,
		}
	}

	inflater, err := GetByType(c.Type, c.Subtype, byteConfig)
	if err != nil {
		logger.Error("mount_config: could not create", "id", c.Id, "error", err)
		return &ParsedMount{
			Valid: false,
			Conf:  c,
		}
	}

	return &ParsedMount{
		Valid:   true,
		Conf:    c,
		Inflate: inflater,
	}
}

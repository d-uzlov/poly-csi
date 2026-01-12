package mpfactory

import (
	"context"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/multipath"
)

type MultipathConfig struct {
	Id      string         `yaml:"id"`
	Type    string         `yaml:"type"`
	Subtype string         `yaml:"subtype"`
	Config  map[string]any `yaml:"config"`
}

type ParsedMultipath struct {
	Valid   bool
	Inflate func() multipath.Multipath
	Conf    *MultipathConfig
}

func ParseMultipath(ctx context.Context, c *MultipathConfig) *ParsedMultipath {
	logger := slogctx.FromCtx(ctx)

	if c.Type == "" {
		logger.Error("mount_config: missing type", "id", c.Id)
		return &ParsedMultipath{
			Valid: false,
			Conf:  c,
		}
	}

	byteConfig, err := yaml.Marshal(c.Config)
	if err != nil {
		logger.Error("mount_config: marshal config", "id", c.Id, "error", err)
		return &ParsedMultipath{
			Valid: false,
			Conf:  c,
		}
	}

	inflater, err := GetByType(c.Type, c.Subtype, byteConfig)
	if err != nil {
		logger.Error("mount_config: could not create", "id", c.Id, "error", err)
		return &ParsedMultipath{
			Valid: false,
			Conf:  c,
		}
	}

	return &ParsedMultipath{
		Valid:   true,
		Conf:    c,
		Inflate: inflater,
	}
}

package mpfactory

import (
	"context"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/transport"
)

type MultipathConfig struct {
	Id      string         `yaml:"id"`
	Type    string         `yaml:"type"`
	Subtype string         `yaml:"subtype"`
	Config  map[string]any `yaml:"config"`
}

type ParsedMultipath struct {
	Valid   bool
	Inflate func(tr transport.Transport) multipath.Multipath
	Conf    *MultipathConfig
}

func ParseMultipath(ctx context.Context, c *MultipathConfig, devFolder string) *ParsedMultipath {
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
		logger.Error("mount_config: marshal config", "id", c.Id, "error", err.Error())
		return &ParsedMultipath{
			Valid: false,
			Conf:  c,
		}
	}

	inflater, err := GetByType(c.Type, c.Subtype, byteConfig, devFolder)
	if err != nil {
		logger.Error("mount_config: could not create", "id", c.Id, "error", err.Error())
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

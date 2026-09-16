package connectorfactory

import (
	"context"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/connectors"
)

type ConnectorSpec struct {
	Id      string         `yaml:"id"`
	Type    string         `yaml:"type"`
	Subtype string         `yaml:"subtype"`
	Config  map[string]any `yaml:"config"`

	ByteConfig []byte `yaml:"-"`
}

type ParsedConnector struct {
	Valid   bool
	Inflate connectors.InflateFunc
	Conf    *ConnectorSpec
}

func ParseConnector(ctx context.Context, c *ConnectorSpec) *ParsedConnector {
	logger := slogctx.FromCtx(ctx)

	if c.Type == "" {
		logger.Error("mount_config: missing type", "id", c.Id)
		return &ParsedConnector{
			Valid: false,
			Conf:  c,
		}
	}

	byteConfig, err := yaml.Marshal(c.Config)
	if err != nil {
		logger.Error("mount_config: marshal config", "id", c.Id, "error", err.Error())
		return &ParsedConnector{
			Valid: false,
			Conf:  c,
		}
	}
	c.ByteConfig = byteConfig

	inflater, err := GetByType(c.Type, c.Subtype)
	if err != nil {
		logger.Error("mount_config: could not create", "id", c.Id, "error", err.Error())
		return &ParsedConnector{
			Valid: false,
			Conf:  c,
		}
	}

	return &ParsedConnector{
		Valid:   true,
		Conf:    c,
		Inflate: inflater,
	}
}

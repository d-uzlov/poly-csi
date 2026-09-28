package mpfactory

import (
	"context"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/transport"
)

type MultipathConfig struct {
	Type   string    `yaml:"type"`
	Config yaml.Node `yaml:"config"`
}

type ParsedMultipath struct {
	Valid   bool
	Inflate func(tr transport.Transport) multipath.Multipath
	Conf    *MultipathConfig
}

func ParseMultipath(ctx context.Context, c *MultipathConfig, devFolder string) *ParsedMultipath {
	logger := slogctx.FromCtx(ctx)

	if c.Type == "" {
		logger.Error("multipath: missing type")
		return &ParsedMultipath{
			Valid: false,
			Conf:  c,
		}
	}

	inflater, err := GetByType(c.Type, c.Config, devFolder)
	if err != nil {
		logger.Error("multipath: GetByType failed", "error", err.Error())
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

package mpfactory

import (
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/node/multipath/mplocal"
	"meoe.io/poly-csi/pkg/node/multipath/mpnone"
	"meoe.io/poly-csi/pkg/transport"
)

const (
	MultipathTypeLocal = "local"
	MultipathTypeNone  = "none"
)

func GetByType(mtype string, byteConfig []byte, devFolder string) (func(tr transport.Transport) multipath.Multipath, error) {
	switch mtype {
	case MultipathTypeLocal:
		var cfg mplocal.Config
		if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s config: %w", mtype, err)
		}
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("validate %s config: %w", mtype, err)
		}
		return func(tr transport.Transport) multipath.Multipath {
			return mplocal.New(tr, cfg, devFolder)
		}, nil
	case MultipathTypeNone:
		return func(tr transport.Transport) multipath.Multipath {
			return mpnone.New()
		}, nil
	}
	return nil, fmt.Errorf("unknown multipath type '%s', must be one of: local, none", mtype)
}

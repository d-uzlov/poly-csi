package mpfactory

import (
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/node/multipath/mplocal"
	"meoe.io/poly-csi/pkg/node/multipath/mpnone"
	"meoe.io/poly-csi/pkg/node/multipath/mpplugin/mphttp"
	"meoe.io/poly-csi/pkg/transport"
)

const (
	MultipathTypeLocal = "local"
	MultipathTypeNone  = "none"
)

func GetByType(mtype string, subtype string, byteConfig []byte, devFolder string) (func(tr transport.Transport) multipath.Multipath, error) {
	if mtype == "builtin" {
		switch subtype {
		case MultipathTypeLocal:
			var cfg mplocal.Config
			if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
				return nil, fmt.Errorf("parse %s/%s config: %w", mtype, subtype, err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("validate %s/%s config: %w", mtype, subtype, err)
			}
			return func(tr transport.Transport) multipath.Multipath {
				return mplocal.NewLocalMp(tr, cfg, devFolder)
			}, nil
		case MultipathTypeNone:
			return func(tr transport.Transport) multipath.Multipath {
				return mpnone.NewNoneMp()
			}, nil
		}
		return nil, fmt.Errorf("unknown multipath subtype: '%s/%s', must be one of: local", mtype, subtype)
	}
	if mtype == "plugin" {
		switch subtype {
		case "http.json.dev":
			var cfg mphttp.ClientConfig
			if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
				return nil, fmt.Errorf("parse %s/%s config: %w", mtype, subtype, err)
			}
			return func(tr transport.Transport) multipath.Multipath {
				return mphttp.NewHttpJsonMultipathPlugin(cfg, devFolder)
			}, nil
		}
		return nil, fmt.Errorf("unknown multipath subtype: '%s/%s', must be one of: http.json.dev", mtype, subtype)
	}
	return nil, fmt.Errorf("unknown multipath type: '%s', must be one of: builtin, plugin", mtype)
}

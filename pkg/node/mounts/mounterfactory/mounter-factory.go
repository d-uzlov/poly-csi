package mounterfactory

import (
	"fmt"
	"maps"
	"slices"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mntriscsiadm"
	"meoe.io/poly-csi/pkg/node/mounts/mntrnfs"
	"meoe.io/poly-csi/pkg/node/mounts/mntrnvme"
	"meoe.io/poly-csi/pkg/node/mounts/mountplugins/mounthttp"
)

const (
	MounterTypeNfs      = "nfs"
	MounterTypeNvme     = "nvme"
	MounterTypeIscsiadm = "iscsiadm"
)

var builtinTypes = map[string]func(byteConfig []byte) (func() mounts.NodeMounter, error){
	MounterTypeNfs: func(byteConfig []byte) (func() mounts.NodeMounter, error) {
		return func() mounts.NodeMounter { return mntrnfs.NewNfsMounter() }, nil
	},
	MounterTypeNvme: func(byteConfig []byte) (func() mounts.NodeMounter, error) {
		var cfg mntrnvme.Config
		if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
		cfg.FillDefaults()
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("validate config: %w", err)
		}
		return func() mounts.NodeMounter { return mntrnvme.NewNvmeMounter(cfg) }, nil
	},
	MounterTypeIscsiadm: func(byteConfig []byte) (func() mounts.NodeMounter, error) {
		var cfg mntriscsiadm.Config
		if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
		cfg.FillDefaults()
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("validate config: %w", err)
		}
		return func() mounts.NodeMounter {
			return mntriscsiadm.NewIscsiadmMounter(cfg)
		}, nil
	},
}

func GetByType(mtype string, subtype string, byteConfig []byte) (func() mounts.NodeMounter, error) {
	if mtype == "builtin" {
		v, ok := builtinTypes[subtype]
		if !ok {
			return nil, fmt.Errorf("unknown mounter subtype: '%s/%s', must be one of: %v", mtype, subtype, slices.Collect(maps.Keys(builtinTypes)))
		}
		return v(byteConfig)
	}
	if mtype == "plugin" {
		switch subtype {
		case "http.json.dev":
			var cfg mounthttp.ClientConfig
			if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
				return nil, fmt.Errorf("parse %s/%s config: %w", mtype, subtype, err)
			}
			return func() mounts.NodeMounter {
				return mounthttp.NewHttpJsonMounterPlugin(cfg)
			}, nil
		}
		return nil, fmt.Errorf("unknown mounter subtype: '%s/%s', must be one of: http.json.dev", mtype, subtype)
	}
	return nil, fmt.Errorf("unknown mounter type: '%s', must be one of: builtin, plugin", mtype)
}

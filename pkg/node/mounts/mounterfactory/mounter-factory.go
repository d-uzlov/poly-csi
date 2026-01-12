package mounterfactory

import (
	"fmt"

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

var allBuiltinTypes = []string{
	MounterTypeNfs,
	MounterTypeNvme,
	MounterTypeIscsiadm,
}

func GetByType(id string, mtype string, subtype string, byteConfig []byte) (func() mounts.NodeMounter, error) {
	if mtype == "builtin" {
		switch subtype {
		case MounterTypeNfs:
			return func() mounts.NodeMounter { return mntrnfs.NewNfsMounter(id) }, nil
		case MounterTypeNvme:
			return func() mounts.NodeMounter { return mntrnvme.NewNvmeMounter(id) }, nil
		case MounterTypeIscsiadm:
			var cfg mntriscsiadm.Config
			if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
				return nil, fmt.Errorf("parse %s/%s config: %w", mtype, subtype, err)
			}
			return func() mounts.NodeMounter {
				return mntriscsiadm.NewIscsiadmMounter(id, cfg)
			}, nil
		}
		return nil, fmt.Errorf("unknown mounter subtype: '%s/%s', must be one of: %v", mtype, subtype, allBuiltinTypes)
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

package connectorfactory

import (
	"fmt"
	"maps"
	"slices"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrcifs"
	"meoe.io/poly-csi/pkg/node/connectors/cntriscsiadm"
	"meoe.io/poly-csi/pkg/node/connectors/cntrnfs"
	"meoe.io/poly-csi/pkg/node/connectors/cntrnvme"
	"meoe.io/poly-csi/pkg/node/mounts/mountplugins/mounthttp"
	"meoe.io/poly-csi/pkg/transport"
)

const (
	ConnectorTypeNfs      = "nfs"
	ConnectorTypeCifs     = "cifs"
	ConnectorTypeNvme     = "nvme"
	ConnectorTypeIscsiadm = "iscsiadm"
)

var builtinTypes = map[string]func(byteConfig []byte, devFolder string) (func(tr transport.Transport) connectors.Connector, error){
	ConnectorTypeNfs: func(byteConfig []byte, devFolder string) (func(tr transport.Transport) connectors.Connector, error) {
		return func(tr transport.Transport) connectors.Connector { return cntrnfs.New(tr) }, nil
	},
	ConnectorTypeCifs: func(byteConfig []byte, devFolder string) (func(tr transport.Transport) connectors.Connector, error) {
		return func(tr transport.Transport) connectors.Connector { return cntrcifs.New(tr) }, nil
	},
	ConnectorTypeNvme: func(byteConfig []byte, devFolder string) (func(tr transport.Transport) connectors.Connector, error) {
		var cfg cntrnvme.Config
		if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
		cfg.FillDefaults()
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("validate config: %w", err)
		}
		return func(tr transport.Transport) connectors.Connector { return cntrnvme.New(tr, cfg, devFolder) }, nil
	},
	ConnectorTypeIscsiadm: func(byteConfig []byte, devFolder string) (func(tr transport.Transport) connectors.Connector, error) {
		var cfg cntriscsiadm.Config
		if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
		cfg.FillDefaults()
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("validate config: %w", err)
		}
		return func(tr transport.Transport) connectors.Connector {
			return cntriscsiadm.New(tr, cfg, devFolder)
		}, nil
	},
}

func GetByType(mtype string, subtype string, byteConfig []byte, devFolder string) (func(tr transport.Transport) connectors.Connector, error) {
	if mtype == "builtin" {
		v, ok := builtinTypes[subtype]
		if !ok {
			return nil, fmt.Errorf("unknown connector subtype: '%s/%s', must be one of: %v", mtype, subtype, slices.Collect(maps.Keys(builtinTypes)))
		}
		return v(byteConfig, devFolder)
	}
	if mtype == "plugin" {
		switch subtype {
		case "http.json.dev":
			var cfg mounthttp.ClientConfig
			if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
				return nil, fmt.Errorf("parse %s/%s config: %w", mtype, subtype, err)
			}
			return func(tr transport.Transport) connectors.Connector {
				return mounthttp.NewHttpJsonConnectorPlugin(cfg, devFolder)
			}, nil
		}
		return nil, fmt.Errorf("unknown connector subtype: '%s/%s', must be one of: http.json.dev", mtype, subtype)
	}
	return nil, fmt.Errorf("unknown connector type: '%s', must be one of: builtin, plugin", mtype)
}

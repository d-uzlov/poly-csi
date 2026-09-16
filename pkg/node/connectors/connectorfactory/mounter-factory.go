package connectorfactory

import (
	"fmt"
	"maps"
	"slices"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrcifs"
	"meoe.io/poly-csi/pkg/node/connectors/cntrhttpjsonplugin"
	"meoe.io/poly-csi/pkg/node/connectors/cntriscsiadm"
	"meoe.io/poly-csi/pkg/node/connectors/cntrnfs"
	"meoe.io/poly-csi/pkg/node/connectors/cntrnvme"
)

const (
	ConnectorTypeNfs      = "nfs"
	ConnectorTypeCifs     = "cifs"
	ConnectorTypeNvme     = "nvme"
	ConnectorTypeIscsiadm = "iscsiadm"
)

var builtinTypes = map[string]func() (connectors.InflateFunc, error){
	ConnectorTypeNfs: func() (connectors.InflateFunc, error) {
		return cntrnfs.New, nil
	},
	ConnectorTypeCifs: func() (connectors.InflateFunc, error) {
		return cntrcifs.New, nil
	},
	ConnectorTypeNvme: func() (connectors.InflateFunc, error) {
		return cntrnvme.New, nil
	},
	ConnectorTypeIscsiadm: func() (connectors.InflateFunc, error) {
		return cntriscsiadm.New, nil
	},
}

func GetByType(mtype string, subtype string) (connectors.InflateFunc, error) {
	if mtype == "builtin" {
		v, ok := builtinTypes[subtype]
		if !ok {
			return nil, fmt.Errorf("unknown connector subtype: '%s/%s', must be one of: %v", mtype, subtype, slices.Collect(maps.Keys(builtinTypes)))
		}
		return v()
	}
	if mtype == "plugin" {
		switch subtype {
		case "http.json.dev":
			return func() connectors.Connector {
				httpClient := cntrhttpjsonplugin.NewHttpClient()
				return cntrhttpjsonplugin.NewDispatcherClient(httpClient)
			}, nil
		}
		return nil, fmt.Errorf("unknown connector subtype: '%s/%s', must be one of: http.json.dev", mtype, subtype)
	}
	return nil, fmt.Errorf("unknown connector type: '%s', must be one of: builtin, plugin", mtype)
}

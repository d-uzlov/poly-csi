package connectorfactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

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
	ConnectorTypePlugin   = "plugin_json"
)

var builtinTypes = map[string]connectors.InflateFunc{
	ConnectorTypeNfs:      cntrnfs.New,
	ConnectorTypeCifs:     cntrcifs.New,
	ConnectorTypeNvme:     cntrnvme.New,
	ConnectorTypeIscsiadm: cntriscsiadm.New,
	ConnectorTypePlugin: func(conf connectors.CommonConfig) (connectors.Manager, error) {
		httpClient := cntrhttpjsonplugin.NewHttpClient()
		return cntrhttpjsonplugin.NewDispatcherClient(httpClient, conf)
	},
}

func GetByType(mtype string) (connectors.InflateFunc, error) {
	v, ok := builtinTypes[mtype]
	if !ok {
		return nil, fmt.Errorf("unknown connector type: '%s', must be one of: %v", mtype, strings.Join(slices.Collect(maps.Keys(builtinTypes)), ", "))
	}
	return v, nil
}

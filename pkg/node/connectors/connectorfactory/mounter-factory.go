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
	TypeNfs      = "nfs"
	TypeCifs     = "cifs"
	TypeNvme     = "nvme"
	TypeIscsiadm = "iscsiadm"
	TypePlugin   = "plugin_json"
)

var builtinTypes = map[string]connectors.InflateFunc{
	TypeNfs:      cntrnfs.New,
	TypeCifs:     cntrcifs.New,
	TypeNvme:     cntrnvme.New,
	TypeIscsiadm: cntriscsiadm.New,
	TypePlugin: func(conf connectors.CommonConfig) (connectors.Manager, error) {
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

package connectorfactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntraoe"
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
	TypeAoe      = "aoe"
	TypePlugin   = "plugin_json"
)

var types = map[string]connectors.InflateFunc{
	TypeNfs:      cntrnfs.New,
	TypeCifs:     cntrcifs.New,
	TypeNvme:     cntrnvme.New,
	TypeIscsiadm: cntriscsiadm.New,
	TypeAoe:      cntraoe.New,
	TypePlugin: func(conf connectors.CommonConfig) (connectors.Manager, error) {
		httpClient := cntrhttpjsonplugin.NewHttpClient()
		return cntrhttpjsonplugin.NewDispatcherClient(httpClient, conf)
	},
}

func GetByType(mtype string) (connectors.InflateFunc, error) {
	v, ok := types[mtype]
	if !ok {
		keys := slices.Collect(maps.Keys(types))
		slices.Sort(keys)
		return nil, fmt.Errorf("unknown connector type: '%s', must be one of: %s", mtype, strings.Join(keys, ", "))
	}
	return v, nil
}

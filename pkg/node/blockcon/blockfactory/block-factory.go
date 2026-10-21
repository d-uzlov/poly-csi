package blockfactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/node/blockcon/cntraoe"
	"meoe.io/poly-csi/pkg/node/blockcon/cntriscsiadm"
	"meoe.io/poly-csi/pkg/node/blockcon/cntrnvme"
)

const (
	TypeNfs      = "nfs"
	TypeCifs     = "cifs"
	TypeNvme     = "nvme"
	TypeIscsiadm = "iscsiadm"
	TypeAoe      = "aoe"
	TypePlugin   = "plugin_json"
)

var types = map[string]blockapi.InflateFunc{
	TypeNvme:     cntrnvme.New,
	TypeIscsiadm: cntriscsiadm.New,
	TypeAoe:      cntraoe.New,
	// TypePlugin: func(conf blockcon.CommonConfig) (blockcon.Manager, error) {
	// 	httpClient := cntrhttpjsonplugin.NewHttpClient()
	// 	return cntrhttpjsonplugin.NewDispatcherClient(httpClient, conf)
	// },
}

func GetByType(mtype string) (blockapi.InflateFunc, error) {
	v, ok := types[mtype]
	if !ok {
		keys := slices.Collect(maps.Keys(types))
		slices.Sort(keys)
		return nil, fmt.Errorf("unknown connector type: '%s', must be one of: %s", mtype, strings.Join(keys, ", "))
	}
	return v, nil
}

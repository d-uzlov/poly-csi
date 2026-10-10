package ublkfactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"meoe.io/poly-csi/pkg/node/blockcon/cntrublk"
	"meoe.io/poly-csi/pkg/node/blockcon/cntrublk/cntrublkiscsi"
	"meoe.io/poly-csi/pkg/node/blockcon/cntrublk/cntrublknbd"
)

const (
	TypeNbd   = "ublk_nbd"
	TypeIscsi = "ublk_iscsi"
)

var types = map[string]cntrublk.InflateFunc{
	TypeNbd:   cntrublknbd.New,
	TypeIscsi: cntrublkiscsi.New,
}

func GetByType(mtype string) (cntrublk.InflateFunc, error) {
	v, ok := types[mtype]
	if !ok {
		keys := slices.Collect(maps.Keys(types))
		slices.Sort(keys)
		return nil, fmt.Errorf("unknown connector type: '%s', must be one of: %s", mtype, strings.Join(keys, ", "))
	}
	return v, nil
}

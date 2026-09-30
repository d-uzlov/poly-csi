package ublkfactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"meoe.io/poly-csi/pkg/node/connectors/cntrublk"
	"meoe.io/poly-csi/pkg/node/connectors/cntrublk/cntrublkiscsi"
	"meoe.io/poly-csi/pkg/node/connectors/cntrublk/cntrublknbd"
)

const (
	TypeNbd   = "ublk_nbd"
	TypeIscsi = "ublk_iscsi"
)

var builtinTypes = map[string]cntrublk.InflateFunc{
	TypeNbd:   cntrublknbd.New,
	TypeIscsi: cntrublkiscsi.New,
}

func GetByType(mtype string) (cntrublk.InflateFunc, error) {
	v, ok := builtinTypes[mtype]
	if !ok {
		return nil, fmt.Errorf("unknown connector type: '%s', must be one of: %v", mtype, strings.Join(slices.Collect(maps.Keys(builtinTypes)), ", "))
	}
	return v, nil
}

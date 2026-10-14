package blkvolfactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"meoe.io/poly-csi/pkg/controller/blockvolume"
	"meoe.io/poly-csi/pkg/controller/blockvolume/strlvm"
	"meoe.io/poly-csi/pkg/controller/blockvolume/strzfszvol"
	"meoe.io/poly-csi/pkg/transport"
)

const (
	VolumeManagerZfsZvol  = "zfs-zvol"
	VolumeManagerLvmThick = "lvm-thick"
)

type InflateFunc func(tr transport.Transport, globalParams map[string]string) (blockvolume.Manager, error)

var types = map[string]InflateFunc{
	VolumeManagerZfsZvol:  strzfszvol.New,
	VolumeManagerLvmThick: strlvm.New,
}

func StorageFromType(tr transport.Transport, vmType string, globalParams map[string]string) (blockvolume.Manager, error) {
	v, ok := types[vmType]
	if !ok {
		keys := slices.Collect(maps.Keys(types))
		slices.Sort(keys)
		return nil, fmt.Errorf("unknown storage type: '%s', must be one of: %s", vmType, strings.Join(keys, ", "))
	}
	return v(tr, globalParams)
}

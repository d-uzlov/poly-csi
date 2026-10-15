package filesharefactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"meoe.io/poly-csi/pkg/controller/fileshare"
	"meoe.io/poly-csi/pkg/controller/fileshare/shrnfs"
	"meoe.io/poly-csi/pkg/controller/fileshare/shrsamba"
	"meoe.io/poly-csi/pkg/controller/fileshare/shrtnnfs"
	"meoe.io/poly-csi/pkg/controller/fileshare/shrzfssharenfs"
	"meoe.io/poly-csi/pkg/transport"
)

const (
	shareZfsSharenfs = "zfs-sharenfs"
	shareNfs         = "nfs-exports"
	shareTnNfs       = "truenas-nfs"
	shareSamba       = "samba"
)

type InflateFunc func(tr transport.Transport, globalParams map[string]string) (fileshare.Manager, error)

var types = map[string]InflateFunc{
	shareZfsSharenfs: shrzfssharenfs.New,
	shareNfs:         shrnfs.New,
	shareTnNfs:       shrtnnfs.New,
	shareSamba:       shrsamba.New,
}

func ShareFromType(tr transport.Transport, vmType string, globalParams map[string]string) (fileshare.Manager, error) {
	v, ok := types[vmType]
	if !ok {
		keys := slices.Collect(maps.Keys(types))
		slices.Sort(keys)
		return nil, fmt.Errorf("unknown share type: '%s', must be one of: %s", vmType, strings.Join(keys, ", "))
	}
	return v(tr, globalParams)
}

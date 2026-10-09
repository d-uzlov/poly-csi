package sharefactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/shares/shrnbd"
	"meoe.io/poly-csi/pkg/controller/shares/shrnfs"
	"meoe.io/poly-csi/pkg/controller/shares/shrnvmetkeeper"
	"meoe.io/poly-csi/pkg/controller/shares/shrnvmetkernel"
	"meoe.io/poly-csi/pkg/controller/shares/shrsamba"
	"meoe.io/poly-csi/pkg/controller/shares/shrtargetcli"
	"meoe.io/poly-csi/pkg/controller/shares/shrvblade"
	"meoe.io/poly-csi/pkg/controller/shares/shrzfssharenfs"
	"meoe.io/poly-csi/pkg/controller/shares/truenas/shrtniscsi"
	"meoe.io/poly-csi/pkg/controller/shares/truenas/shrtnnfs"
	"meoe.io/poly-csi/pkg/transport"
)

const (
	shareNvmetKernel = "nvmet-kernel"
	shareNvmetKeeper = "nvmet-keeper"
	shareZfsSharenfs = "zfs-sharenfs"
	shareNfs         = "nfs-exports"
	shareTargetcli   = "targetcli"
	shareTnNfs       = "truenas-nfs"
	shareTnIscsi     = "truenas-iscsi"
	shareSamba       = "samba"
	shareNbd         = "nbd"
	shareVblade      = "vblade"
)

type InflateFunc func(tr transport.Transport, globalParams map[string]string) (shares.ShareManager, error)

var types = map[string]InflateFunc{
	shareNvmetKernel: shrnvmetkernel.New,
	shareNvmetKeeper: shrnvmetkeeper.New,
	shareZfsSharenfs: shrzfssharenfs.New,
	shareNfs:         shrnfs.New,
	shareTargetcli:   shrtargetcli.New,
	shareTnNfs:       shrtnnfs.New,
	shareTnIscsi:     shrtniscsi.New,
	shareSamba:       shrsamba.New,
	shareNbd:         shrnbd.New,
	shareVblade:      shrvblade.New,
}

func ShareFromType(tr transport.Transport, vmType string, globalParams map[string]string) (shares.ShareManager, error) {
	v, ok := types[vmType]
	if !ok {
		keys := slices.Collect(maps.Keys(types))
		slices.Sort(keys)
		return nil, fmt.Errorf("unknown share type: '%s', must be one of: %s", vmType, strings.Join(keys, ", "))
	}
	return v(tr, globalParams)
}

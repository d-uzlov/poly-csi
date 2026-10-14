package blkshrfactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"meoe.io/poly-csi/pkg/controller/blockshare"
	"meoe.io/poly-csi/pkg/controller/blockshare/shrnbd"
	"meoe.io/poly-csi/pkg/controller/blockshare/shrnvmetkeeper"
	"meoe.io/poly-csi/pkg/controller/blockshare/shrnvmetkernel"
	"meoe.io/poly-csi/pkg/controller/blockshare/shrtargetcli"
	"meoe.io/poly-csi/pkg/controller/blockshare/shrtniscsi"
	"meoe.io/poly-csi/pkg/controller/blockshare/shrvblade"
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

type InflateFunc func(tr transport.Transport, globalParams map[string]string) (blockshare.Manager, error)

var types = map[string]InflateFunc{
	shareNvmetKernel: shrnvmetkernel.New,
	shareNvmetKeeper: shrnvmetkeeper.New,
	shareTargetcli:   shrtargetcli.New,
	shareTnIscsi:     shrtniscsi.New,
	shareNbd:         shrnbd.New,
	shareVblade:      shrvblade.New,
	// shareZfsSharenfs: shrzfssharenfs.New,
	// shareNfs:         shrnfs.New,
	// shareTnNfs:       shrtnnfs.New,
	// shareSamba:       shrsamba.New,
}

func ShareFromType(tr transport.Transport, vmType string, globalParams map[string]string) (blockshare.Manager, error) {
	v, ok := types[vmType]
	if !ok {
		keys := slices.Collect(maps.Keys(types))
		slices.Sort(keys)
		return nil, fmt.Errorf("unknown share type: '%s', must be one of: %s", vmType, strings.Join(keys, ", "))
	}
	return v(tr, globalParams)
}

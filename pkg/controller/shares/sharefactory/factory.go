package sharefactory

import (
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/shares/shrnfs"
	"meoe.io/poly-csi/pkg/controller/shares/shrnvmetkeeper"
	"meoe.io/poly-csi/pkg/controller/shares/shrnvmetkernel"
	"meoe.io/poly-csi/pkg/controller/shares/shrsamba"
	"meoe.io/poly-csi/pkg/controller/shares/shrtargetcli"
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
)

func ShareFromType(tr transport.Transport, vmType string, commands map[string][]string, globalParams map[string]string) (shares.ShareManager, error) {
	switch vmType {
	case shareZfsSharenfs:
		return shrzfssharenfs.New(tr, commands, globalParams)
	case shareNfs:
		return shrnfs.New(tr, commands, globalParams)
	case shareNvmetKernel:
		return shrnvmetkernel.New(tr, commands, globalParams)
	case shareNvmetKeeper:
		return shrnvmetkeeper.New(tr, commands, globalParams)
	case shareTargetcli:
		return shrtargetcli.New(tr, commands, globalParams)
	case shareTnNfs:
		return shrtnnfs.New(tr, commands, globalParams)
	case shareTnIscsi:
		return shrtniscsi.New(tr, commands, globalParams)
	case shareSamba:
		return shrsamba.New(tr, commands, globalParams)
	}
	return nil, fmt.Errorf("unknown share type: %s", vmType)
}

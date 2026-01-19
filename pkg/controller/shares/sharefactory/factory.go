package sharefactory

import (
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/shares/shrnfs"
	"meoe.io/poly-csi/pkg/controller/shares/shrnvmetkeeper"
	"meoe.io/poly-csi/pkg/controller/shares/shrnvmetkernel"
	"meoe.io/poly-csi/pkg/controller/shares/shrtargetcli"
	"meoe.io/poly-csi/pkg/controller/shares/shrzfssharenfs"
	"meoe.io/poly-csi/pkg/controller/shares/truenas/shrtniscsi"
	"meoe.io/poly-csi/pkg/controller/shares/truenas/shrtnnfs"
)

const (
	shareNvmetKernel = "nvmet-kernel"
	shareNvmetKeeper = "nvmet-keeper"
	shareZfsSharenfs = "zfs-sharenfs"
	shareNfs         = "nfs-exports"
	shareTargetcli   = "targetcli"
	shareTnNfs       = "truenas-nfs"
	shareTnIscsi     = "truenas-iscsi"
)

func ShareFromType(vmType string, commands map[string][]string, globalParams map[string]string) (shares.ShareManager, error) {
	switch vmType {
	case shareZfsSharenfs:
		return shrzfssharenfs.New(commands, globalParams)
	case shareNfs:
		return shrnfs.New(commands, globalParams)
	case shareNvmetKernel:
		return shrnvmetkernel.New(commands, globalParams)
	case shareNvmetKeeper:
		return shrnvmetkeeper.New(commands, globalParams)
	case shareTargetcli:
		return shrtargetcli.New(commands, globalParams)
	case shareTnNfs:
		return shrtnnfs.New(commands, globalParams)
	case shareTnIscsi:
		return shrtniscsi.New(commands, globalParams)
	}
	return nil, fmt.Errorf("unknown share type: %s", vmType)
}

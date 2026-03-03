package storagefactory

import (
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/strbtrfs"
	"meoe.io/poly-csi/pkg/controller/storage/strfolder"
	"meoe.io/poly-csi/pkg/controller/storage/strlvm"
	"meoe.io/poly-csi/pkg/controller/storage/strzfsdataset"
	"meoe.io/poly-csi/pkg/controller/storage/strzfszvol"
	"meoe.io/poly-csi/pkg/transport"
)

const (
	VolumeManagerZfsDataset = "zfs-dataset"
	VolumeManagerZfsZvol    = "zfs-zvol"
	VolumeManagerFolder     = "folder"
	VolumeManagerBtrfs      = "btrfs"
	VolumeManagerLvmThick   = "lvm-thick"
)

func StorageFromType(tr transport.Transport, vmType string, commands map[string][]string, globalParams map[string]string) (storage.StorageManager, error) {
	switch vmType {
	case VolumeManagerZfsDataset:
		return strzfsdataset.New(tr, commands, globalParams)
	case VolumeManagerZfsZvol:
		return strzfszvol.New(tr, commands, globalParams)
	case VolumeManagerFolder:
		return strfolder.New(tr, commands, globalParams), nil
	case VolumeManagerBtrfs:
		return strbtrfs.New(tr, commands, globalParams)
	case VolumeManagerLvmThick:
		return strlvm.New(tr, commands, globalParams)
	}
	return nil, fmt.Errorf("unknown storage type: %s", vmType)
}

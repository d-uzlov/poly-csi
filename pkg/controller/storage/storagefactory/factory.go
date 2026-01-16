package storagefactory

import (
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/strbtrfs"
	"meoe.io/poly-csi/pkg/controller/storage/strfolder"
	"meoe.io/poly-csi/pkg/controller/storage/strlvm"
	"meoe.io/poly-csi/pkg/controller/storage/strzfsdataset"
	"meoe.io/poly-csi/pkg/controller/storage/strzfszvol"
)

const (
	VolumeManagerZfsDataset = "zfs-dataset"
	VolumeManagerZfsZvol    = "zfs-zvol"
	VolumeManagerFolder     = "folder"
	VolumeManagerBtrfs      = "btrfs"
	VolumeManagerLvmThick   = "lvm-thick"
)

func StorageFromType(vmType string, commands map[string][]string, globalParams map[string]string) (storage.StorageManager, error) {
	switch vmType {
	case VolumeManagerZfsDataset:
		return strzfsdataset.New(commands, globalParams)
	case VolumeManagerZfsZvol:
		return strzfszvol.New(commands, globalParams)
	case VolumeManagerFolder:
		return strfolder.New(commands, globalParams), nil
	case VolumeManagerBtrfs:
		return strbtrfs.New(commands, globalParams)
	case VolumeManagerLvmThick:
		return strlvm.New(commands, globalParams)
	}
	return nil, fmt.Errorf("unknown storage type: %s", vmType)
}

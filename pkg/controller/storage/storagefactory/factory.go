package storagefactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

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

type InflateFunc func(tr transport.Transport, globalParams map[string]string) (storage.StorageManager, error)

var types = map[string]InflateFunc{
	VolumeManagerZfsDataset: strzfsdataset.New,
	VolumeManagerZfsZvol:    strzfszvol.New,
	VolumeManagerFolder:     strfolder.New,
	VolumeManagerBtrfs:      strbtrfs.New,
	VolumeManagerLvmThick:   strlvm.New,
}

func StorageFromType(tr transport.Transport, vmType string, globalParams map[string]string) (storage.StorageManager, error) {
	v, ok := types[vmType]
	if !ok {
		keys := slices.Collect(maps.Keys(types))
		slices.Sort(keys)
		return nil, fmt.Errorf("unknown storage type: '%s', must be one of: %s", vmType, strings.Join(keys, ", "))
	}
	return v(tr, globalParams)
}

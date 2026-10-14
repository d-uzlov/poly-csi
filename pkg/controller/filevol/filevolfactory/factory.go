package filevolfactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"meoe.io/poly-csi/pkg/controller/filevol"
	"meoe.io/poly-csi/pkg/controller/filevol/strbtrfs"
	"meoe.io/poly-csi/pkg/controller/filevol/strfolder"
	"meoe.io/poly-csi/pkg/controller/filevol/strzfsdataset"
	"meoe.io/poly-csi/pkg/transport"
)

const (
	VolumeManagerZfsDataset = "zfs-dataset"
	VolumeManagerFolder     = "folder"
	VolumeManagerBtrfs      = "btrfs"
)

type InflateFunc func(tr transport.Transport, globalParams map[string]string) (filevol.Manager, error)

var types = map[string]InflateFunc{
	VolumeManagerZfsDataset: strzfsdataset.New,
	VolumeManagerFolder:     strfolder.New,
	VolumeManagerBtrfs:      strbtrfs.New,
}

func StorageFromType(tr transport.Transport, vmType string, globalParams map[string]string) (filevol.Manager, error) {
	v, ok := types[vmType]
	if !ok {
		keys := slices.Collect(maps.Keys(types))
		slices.Sort(keys)
		return nil, fmt.Errorf("unknown storage type: '%s', must be one of: %s", vmType, strings.Join(keys, ", "))
	}
	return v(tr, globalParams)
}

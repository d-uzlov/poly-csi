package fsfactory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"meoe.io/poly-csi/pkg/node/filecon"
	"meoe.io/poly-csi/pkg/node/filecon/fscifs"
	"meoe.io/poly-csi/pkg/node/filecon/fsext4"
	"meoe.io/poly-csi/pkg/node/filecon/fsnfs"
	"meoe.io/poly-csi/pkg/node/filecon/fsxfs"
)

const (
	TypeNfs  = "nfs"
	TypeCifs = "cifs"
	TypeExt4 = "ext4"
	TypeXfs  = "xfs"
)

type FsInfo struct {
	FsNew filecon.InflateFunc
	Meta  filecon.MetadataInflateFunc
}

var types = map[string]FsInfo{
	TypeNfs:  FsInfo{FsNew: fsnfs.New},
	TypeCifs: FsInfo{FsNew: fscifs.New},
	TypeExt4: FsInfo{FsNew: fsext4.New, Meta: fsext4.NewMetadata},
	TypeXfs:  FsInfo{FsNew: fsxfs.New, Meta: fsxfs.NewMetadata},
}

func GetByType(mtype string) (FsInfo, error) {
	v, ok := types[mtype]
	if !ok {
		keys := slices.Collect(maps.Keys(types))
		slices.Sort(keys)
		return FsInfo{}, fmt.Errorf("unknown connector type: '%s', must be one of: %s", mtype, strings.Join(keys, ", "))
	}
	return v, nil
}

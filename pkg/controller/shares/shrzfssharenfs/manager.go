package shrzfssharenfs

import (
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
)

type manager struct {
	tr transport.Transport
}

func New(tr transport.Transport, globalParams map[string]string) (shares.ShareManager, error) {
	return &manager{
		tr: tr,
	}, nil
}

func (m *manager) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: true,
		LocalVolume:  []string{storage.VolFolder},
		OpaqueVolume: []string{"zfs-dataset"},
	}
}

func (m *manager) CreateShare(req shares.CreateShareRequest) (shares.Share, error) {
	return nil, fmt.Errorf("unimplemented")
	// p, err := parseParams(req.StaticParams, req.MutableParams)
	// if err != nil {
	// 	return nil, fmt.Errorf("parse params: %w", err)
	// }

	// zfsVol, ok := req.Volume.(*strzfsdataset.Volume)
	// if !ok {
	// 	return nil, fmt.Errorf("storage volume is not a zfs dataset: %T", req.Volume)
	// }

	// c := share{
	// 	tr:      m.tr,
	// 	Dataset: zfsVol.Path,
	// 	Folder:  req.FsPath,
	// }

	// return &c, nil
}

func (m *manager) ParseShare(shareData string) (shares.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
	}
	return result, err
}

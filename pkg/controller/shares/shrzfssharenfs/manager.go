package shrzfssharenfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/strzfsdataset"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/zfs"
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

func (m *manager) CreateShare(ctx context.Context, req shares.CreateShareRequest) (shares.Share, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	zfsVol, ok := req.Volume.(*strzfsdataset.Volume)
	if !ok {
		return nil, fmt.Errorf("storage volume is not a zfs dataset: %T", req.Volume)
	}

	mountPath, err := req.Volume.GetLocalPath(ctx)
	if err != nil {
		return nil, fmt.Errorf("get local path: %w", err)
	}

	snip := zfs.NewSnippets(m.tr)

	err = snip.SetShareNfs(ctx, zfsVol.Path, p.ShareOptions)
	if err != nil {
		return nil, err
	}

	c := share{
		tr:      m.tr,
		Dataset: zfsVol.Path,
		Folder:  mountPath,
	}

	return &c, nil
}

func (m *manager) ParseShare(shareData string) (shares.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
	}
	return result, err
}

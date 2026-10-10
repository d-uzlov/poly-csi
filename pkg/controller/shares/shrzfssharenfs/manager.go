package shrzfssharenfs

import (
	"fmt"

	"meoe.io/poly-csi/pkg/api/protocols"
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
		ProtocolName: protocols.Nfs,
		UnpublishAll: true,
		LocalVolume:  storage.VolFolder,
		OpaqueVolume: "zfs-dataset",
	}
}

func (m *manager) CreateShare(req shares.CreateShareRequest) (shares.Share, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	return &share{
		tr:           m.tr,
		ShareOptions: p.ShareOptions,
	}, nil
}

func (m *manager) ParseShare(shareData string) (shares.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
	}
	return result, err
}

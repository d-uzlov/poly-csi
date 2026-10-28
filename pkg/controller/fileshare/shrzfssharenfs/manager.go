package shrzfssharenfs

import (
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/fsspec"
	"meoe.io/poly-csi/pkg/controller/fileshare"
	"meoe.io/poly-csi/pkg/transport"
)

type manager struct {
	tr transport.Transport
}

func New(tr transport.Transport, globalParams map[string]string) (fileshare.Manager, error) {
	return &manager{
		tr: tr,
	}, nil
}

func (m *manager) GetCapabilities() fileshare.Capabilities {
	return fileshare.Capabilities{
		ProtocolName: fsspec.Nfs,
		UnpublishAll: true,
		OpaqueVolume: "zfs-dataset",
	}
}

func (m *manager) CreateShare(req fileshare.CreateShareRequest) (fileshare.Share, error) {
	p, err := parseParams(req.Parser)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	return &share{
		tr:           m.tr,
		ShareOptions: p.ShareOptions,
	}, nil
}

func (m *manager) ParseShare(shareData json.RawMessage) (fileshare.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
	}
	return result, err
}

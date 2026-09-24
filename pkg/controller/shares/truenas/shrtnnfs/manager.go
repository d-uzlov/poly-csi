package shrtnnfs

import (
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

func (sh *manager) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: true,
		LocalVolume:  []string{storage.VolFolder},
		OpaqueVolume: nil,
	}
}

func (m *manager) CreateShare(req shares.CreateShareRequest) (shares.Share, error) {
	return &share{
		tr:     m.tr,
	}, nil
}

func (m *manager) ParseShare(shareData string) (shares.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
	}
	return result, err
}

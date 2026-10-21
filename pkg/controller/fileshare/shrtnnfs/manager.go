package shrtnnfs

import (
	"encoding/json"

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

func (sh *manager) GetCapabilities() fileshare.Capabilities {
	return fileshare.Capabilities{
		ProtocolName: fsspec.Nfs,
		UnpublishAll: true,
		OpaqueVolume: "",
	}
}

func (m *manager) CreateShare(req fileshare.CreateShareRequest) (fileshare.Share, error) {
	return &share{
		tr: m.tr,
	}, nil
}

func (m *manager) ParseShare(shareData json.RawMessage) (fileshare.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
	}
	return result, err
}

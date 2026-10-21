package shrsamba

import (
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/fsspec"
	"meoe.io/poly-csi/pkg/controller/fileshare"
	"meoe.io/poly-csi/pkg/transport"
)

type manager struct {
	tr transport.Transport
	gp globalParams
}

func New(tr transport.Transport, globalParams map[string]string) (fileshare.Manager, error) {
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	return &manager{
		tr: tr,
		gp: gp,
	}, nil
}

func (m *manager) GetCapabilities() fileshare.Capabilities {
	return fileshare.Capabilities{
		ProtocolName: fsspec.Smb,
		UnpublishAll: true,
		OpaqueVolume: "",
	}
}

func (m *manager) CreateShare(req fileshare.CreateShareRequest) (fileshare.Share, error) {
	_, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	sc := share{
		tr:        m.tr,
		gp:        m.gp,
		ShareName: req.ShareId,
	}
	return &sc, nil
}

func (m *manager) ParseShare(shareData json.RawMessage) (fileshare.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
		result.gp = m.gp
	}
	return result, err
}

package shrnvmetkernel

import (
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/controller/blockshare"
	"meoe.io/poly-csi/pkg/transport"
)

type manager struct {
	tr transport.Transport
	gp globalParams
}

func New(tr transport.Transport, globalParams map[string]string) (blockshare.Manager, error) {
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	return &manager{
		tr: tr,
		gp: gp,
	}, nil
}

func (m *manager) GetCapabilities() blockshare.Capabilities {
	return blockshare.Capabilities{
		ProtocolName: blockspec.Nvme,
		UnpublishAll: false,
		OpaqueVolume: "",
		MultiShare:   true,
	}
}

func (m *manager) CreateShare(req blockshare.CreateShareRequest) (blockshare.Share, error) {
	p, err := parseParams(req.Parser)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	nqn := fmt.Sprintf("%s:%s", m.gp.nqnBase, req.ShareId)

	return &share{
		tr:        m.tr,
		Nqn:       nqn,
		Port:      p.ConnectPort,
		Transport: p.TransportType,
	}, nil
}

func (m *manager) ParseShare(shareData json.RawMessage) (blockshare.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
	}
	return result, err
}

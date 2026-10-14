package shrtargetcli

import (
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockapi"
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
		ProtocolName: blockapi.Iscsi,
		UnpublishAll: false,
		OpaqueVolume: "",
	}
}

func (m *manager) CreateShare(req blockshare.CreateShareRequest) (blockshare.Share, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	err = p.validate()
	if err != nil {
		return nil, fmt.Errorf("validate params: %w", err)
	}

	iqn := fmt.Sprintf("%s:%s", m.gp.iqnBase, req.ShareId)

	return &share{
		tr:        m.tr,
		gp:        m.gp,
		Iqn:       iqn,
		Port:      p.ListenPort,
		Transport: p.Transport,
		AuthType:  p.AuthType,
	}, nil
}

func (m *manager) ParseShare(shareData string) (blockshare.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
		result.gp = m.gp
	}
	return result, err
}

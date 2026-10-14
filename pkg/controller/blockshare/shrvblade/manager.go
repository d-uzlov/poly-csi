package shrvblade

import (
	"encoding/json"
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
		ProtocolName: blockapi.Aoe,
		UnpublishAll: true,
		OpaqueVolume: "",
		MultiShare:   false,
	}
}

func (m *manager) CreateShare(req blockshare.CreateShareRequest) (blockshare.Share, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	return &share{
		tr:   m.tr,
		gp:   m.gp,
		P:    p,
		Name: req.ShareId,
	}, nil
}

func (m *manager) ParseShare(shareData json.RawMessage) (blockshare.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
		result.gp = m.gp
	}
	return result, err
}

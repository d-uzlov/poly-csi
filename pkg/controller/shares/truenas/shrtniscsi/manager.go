package shrtniscsi

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/strzfszvol"
	"meoe.io/poly-csi/pkg/transport"
)

type manager struct {
	tr transport.Transport
	gp globalParams
}

func New(tr transport.Transport, globalParams map[string]string) (shares.ShareManager, error) {
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	return &manager{
		tr: tr,
		gp: gp,
	}, nil
}

func (sh *manager) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: false,
		LocalVolume:  []string{storage.VolDevice},
		OpaqueVolume: []string{"zfs-zvol"},
	}
}

func (m *manager) CreateShare(ctx context.Context, req shares.CreateShareRequest) (shares.Share, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	_ = p

	zfsVol, ok := req.Volume.(*strzfszvol.Volume)
	if !ok {
		return nil, fmt.Errorf("storage volume is not zfs zvol: %T", req.Volume)
	}

	iqn := fmt.Sprintf("%s:%s", m.gp.iqnBase, req.ShareId)

	snip := newSnippets(m.tr)

	err = snip.setupTarget(ctx, tnIscsiRequest{
		extentName: uuid.New().String(),
		iqn:        iqn,
		zvol:       zfsVol.Path,
	})
	if err != nil {
		return nil, err
	}

	c := share{
		tr:   m.tr,
		gp:   m.gp,
		Iqn:  iqn,
		Zvol: zfsVol.Path,
	}

	return &c, nil
}

func (m *manager) ParseShare(shareData string) (shares.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
		result.gp = m.gp
	}
	return result, err
}

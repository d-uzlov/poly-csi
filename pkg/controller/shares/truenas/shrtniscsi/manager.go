package shrtniscsi

import (
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
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

func (m *manager) CreateShare(req shares.CreateShareRequest) (shares.Share, error) {
	return nil, fmt.Errorf("unimplemented")
	// p, err := parseParams(req.StaticParams, req.MutableParams)
	// if err != nil {
	// 	return nil, fmt.Errorf("parse params: %w", err)
	// }
	// _ = p

	// return &share{
	// 	tr:   m.tr,
	// 	gp:   m.gp,
	// 	Iqn:  iqn,
	// 	Zvol: zfsVol.Path,
	// }, nil
}

func (m *manager) ParseShare(shareData string) (shares.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
		result.gp = m.gp
	}
	return result, err
}

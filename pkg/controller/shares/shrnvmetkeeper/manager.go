package shrnvmetkeeper

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/nvmet"
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

func (m *manager) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: false,
		LocalVolume:  []string{storage.VolDevice},
		OpaqueVolume: nil,
	}
}

func (m *manager) CreateShare(ctx context.Context, req shares.CreateShareRequest) (shares.Share, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	nqn := fmt.Sprintf("%s:%s", m.gp.nqnBase, req.ShareId)

	allowAnyHost := false
	if p.AuthType == authTypeNone {
		allowAnyHost = true
	}

	cmd := append(nvmetKeeperCmd, "subsys", "create", nvmet.Subsys{
		Nqn:          nqn,
		AllowAnyHost: allowAnyHost,
	}.Json())
	err = transport.CheckedRun(ctx, m.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("subsys create: %w", err)
	}

	cmd = append(nvmetKeeperCmd, "namespace", "create", nvmet.Ns{
		Nqn:        nqn,
		Nsid:       "1",
		DevicePath: req.FsPath,
		Enabled:    true,
	}.Json())
	err = transport.CheckedRun(ctx, m.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("namespace create: %w", err)
	}

	cmd = append(nvmetKeeperCmd, "subsys", "attach", nqn, p.NvmetPortName)
	err = transport.CheckedRun(ctx, m.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("subsys attach: %w", err)
	}

	sh := share{
		tr:        m.tr,
		Nqn:       nqn,
		Port:      p.ConnectPort,
		Transport: p.TransportType,
		IoQueues:  p.IoQueues,
		AuthType:  p.AuthType,
	}

	return &sh, nil
}

func (m *manager) ParseShare(shareData string) (shares.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
	}
	return result, err
}

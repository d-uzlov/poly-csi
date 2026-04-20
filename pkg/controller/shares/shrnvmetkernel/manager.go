package shrnvmetkernel

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/logs"
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
	if !testReady(ctx, m.tr) {
		return nil, fmt.Errorf("nvmet is not ready")
	}
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	logs.FromCtx(ctx).Args("args: nvmet share", "params", p)

	devicePath, err := req.Volume.GetLocalPath(ctx)
	if err != nil {
		return nil, fmt.Errorf("get local path: %w", err)
	}

	nqn := fmt.Sprintf("%s:%s", m.gp.nqnBase, req.ShareId)

	snip := nvmet.NewSnippets(ctx, m.tr, true)

	err = snip.ConfigureSubsys(nvmet.Subsys{
		Nqn:          nqn,
		AllowAnyHost: false,
	})
	if err != nil {
		return nil, fmt.Errorf("configure subsystem: %w", err)
	}
	err = snip.ConfigureNamespace(nvmet.Ns{
		Nqn:        nqn,
		Nsid:       "1",
		DevicePath: devicePath,
		Enabled:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("configure namespace: %w", err)
	}
	err = snip.AttachSubsysToPort(nqn, "1")
	if err != nil {
		return nil, fmt.Errorf("attach port: %w", err)
	}

	err = saveState(ctx, m.tr)
	if err != nil {
		return nil, fmt.Errorf("nvmet_save: %w", err)
	}

	c := share{
		tr:        m.tr,
		Nqn:       nqn,
		Port:      p.ConnectPort,
		Transport: p.TransportType,
		IoQueues:  p.IoQueues,
	}

	return &c, nil
}

func (m *manager) ParseShare(shareData string) (shares.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
	}
	return result, err
}

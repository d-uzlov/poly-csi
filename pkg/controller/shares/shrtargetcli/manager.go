package shrtargetcli

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/logs"
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

func (m *manager) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: false,
		LocalVolume:  []string{storage.VolDevice},
		OpaqueVolume: nil,
	}
}

func (m *manager) CreateShare(ctx context.Context, req shares.CreateShareRequest) (shares.Share, error) {
	if !testReady(ctx, m.tr) {
		return nil, fmt.Errorf("targetcli is not ready")
	}
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	logs.FromCtx(ctx).Args("args: nvmet share", "params", p)
	err = p.validate()
	if err != nil {
		return nil, fmt.Errorf("validate params: %w", err)
	}

	iqn := fmt.Sprintf("%s:%s", m.gp.iqnBase, req.ShareId)

	snip := newTargetcliSnippets(m.tr)

	// todo seems like backstore names should be short, not iqn
	backstoreName := iqn
	err = snip.configureBackstore(ctx, backstoreName, req.FsPath)
	if err != nil {
		return nil, fmt.Errorf("configure backstore: %w", err)
	}

	enforceAuth := true
	disableAuth := false
	switch p.AuthType {
	case authTypeNone:
		enforceAuth = false
		disableAuth = true
	case authTypeIqn:
		enforceAuth = false
		disableAuth = false
	case authTypeChapProvision:
		enforceAuth = true
		disableAuth = false
	}
	err = snip.configureTarget(ctx, iscsiTargetConfig{
		iqn:           iqn,
		backstoreName: backstoreName,
		auth:          enforceAuth,
		demo:          disableAuth,
	})
	if err != nil {
		return nil, fmt.Errorf("configure target: %w", err)
	}

	err = snip.save(ctx)
	if err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	c := share{
		tr:            m.tr,
		gp:            m.gp,
		Iqn:           iqn,
		Port:          p.ListenPort,
		Transport:     p.Transport,
		BackstoreName: backstoreName,
		AuthType:      p.AuthType,
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

package shrnvmetkeeper

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/nvmet"
)

type share struct {
	tr   transport.Transport
	gp   globalParams
}

func New(tr transport.Transport, globalParams map[string]string) (shares.ShareManager, error) {
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	return &share{
		tr:   tr,
		gp:   gp,
	}, nil
}

func (sh *share) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: false,
		LocalVolume:  []string{storage.VolDevice},
		OpaqueVolume: nil,
	}
}

func (sh *share) CreateShare(ctx context.Context, req shares.CreateShareRequest) (string, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return "", fmt.Errorf("parse params: %w", err)
	}

	devicePath, err := req.Volume.GetLocalPath(ctx)
	if err != nil {
		return "", fmt.Errorf("get local path: %w", err)
	}

	nqn := fmt.Sprintf("%s:%s", sh.gp.nqnBase, req.ShareId)

	allowAnyHost := false
	if p.AuthType == authTypeNone {
		allowAnyHost = true
	}

	cmd := append(nvmetKeeperCmd, "subsys", "create", nvmet.Subsys{
		Nqn:          nqn,
		AllowAnyHost: allowAnyHost,
	}.Json())
	err = transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return "", fmt.Errorf("subsys create: %w", err)
	}

	cmd = append(nvmetKeeperCmd, "namespace", "create", nvmet.Ns{
		Nqn:        nqn,
		Nsid:       "1",
		DevicePath: devicePath,
		Enabled:    true,
	}.Json())
	err = transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return "", fmt.Errorf("namespace create: %w", err)
	}

	cmd = append(nvmetKeeperCmd, "subsys", "attach", nqn, p.NvmetPortName)
	err = transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return "", fmt.Errorf("subsys attach: %w", err)
	}

	c := shareContext{
		Nqn:       nqn,
		Port:      p.ConnectPort,
		Transport: p.TransportType,
		IoQueues:  p.IoQueues,
		AuthType:  p.AuthType,
	}
	err = c.validate()
	if err != nil {
		return "", fmt.Errorf("validate share context: %w", err)
	}

	return c.serialize(), nil
}

func (sh *share) DeleteShare(ctx context.Context, shareData string) error {
	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}

	cmd := append(nvmetKeeperCmd, "subsys", "detach", c.Nqn, c.Port)
	err = transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return fmt.Errorf("subsys detach: %w", err)
	}

	cmd = append(nvmetKeeperCmd, "subsys", "delete", c.Nqn)
	err = transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return fmt.Errorf("subsys delete: %w", err)
	}

	return nil
}

package shrnvmetkernel

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/utils/nvmet"
)

type share struct {
	gp   globalParams
	cmds commands
}

func New(commands map[string][]string, globalParams map[string]string) (shares.ShareManager, error) {
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	cmds, err := parseCommands(commands)
	if err != nil {
		return nil, fmt.Errorf("parse commands: %w", err)
	}
	return &share{
		gp:   gp,
		cmds: cmds,
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
	if !sh.cmds.ready(ctx) {
		return "", fmt.Errorf("nvmet is not ready")
	}
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return "", fmt.Errorf("parse params: %w", err)
	}

	devicePath, err := req.Volume.GetLocalPath(ctx)
	if err != nil {
		return "", fmt.Errorf("get local path: %w", err)
	}

	nqn := fmt.Sprintf("%s:%s", sh.gp.nqnBase, req.ShareId)

	snip := nvmet.NewSnippets(ctx, true)

	err = snip.ConfigureSubsys(nvmet.Subsys{
		Nqn:          nqn,
		AllowAnyHost: false,
	})
	if err != nil {
		return "", fmt.Errorf("configure subsystem: %w", err)
	}
	err = snip.ConfigureNamespace(nvmet.Ns{
		Nqn:        nqn,
		Nsid:       "1",
		DevicePath: devicePath,
		Enabled:    true,
	})
	if err != nil {
		return "", fmt.Errorf("configure namespace: %w", err)
	}
	err = snip.AttachNqnToPort(nqn, "1")
	if err != nil {
		return "", fmt.Errorf("attach port: %w", err)
	}

	err = sh.cmds.save(ctx)
	if err != nil {
		return "", fmt.Errorf("nvmet_save: %w", err)
	}

	c := shareContext{
		Nqn:       nqn,
		Port:      p.listenPort,
		Transport: p.transport,
		IoQueues:  p.ioQueues,
	}
	err = c.validate()
	if err != nil {
		return "", fmt.Errorf("validate share context: %w", err)
	}

	return c.serialize(), nil
}

func (sh *share) DeleteShare(ctx context.Context, shareData string) error {
	if !sh.cmds.ready(ctx) {
		return fmt.Errorf("nvmet is not ready")
	}
	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}

	snip := nvmet.NewSnippets(ctx, true)

	err = snip.DetachNqnFromPort(c.Nqn, "1")
	if err != nil {
		return fmt.Errorf("detach port: %w", err)
	}
	err = snip.DeleteNamespace(c.Nqn, "1")
	if err != nil {
		return fmt.Errorf("delete namespace: %w", err)
	}
	err = snip.DeleteSubsys(c.Nqn)
	if err != nil {
		return fmt.Errorf("delete subsys: %w", err)
	}

	err = sh.cmds.save(ctx)
	if err != nil {
		return fmt.Errorf("nvmet_save: %w", err)
	}

	return nil
}

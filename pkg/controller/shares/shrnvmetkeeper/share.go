package shrnvmetkeeper

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mntrnvme"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/nvmet"
)

type share struct {
	cmds commandsConfig
	gp   globalParams
}

func New(commands map[string][]string, globalParams map[string]string) (shares.ShareHelper, error) {
	cmds, err := parseCommands(commands)
	if err != nil {
		return nil, fmt.Errorf("parse commands: %w", err)
	}
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	return &share{
		cmds: cmds,
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

	cmd := append(sh.cmds.nvmetKeeperCmd, "subsys", "create", nvmet.Subsys{
		Nqn:          nqn,
		AllowAnyHost: false,
	}.Json())
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("subsys create: %w", err)
	}

	cmd = append(sh.cmds.nvmetKeeperCmd, "namespace", "create", nvmet.Ns{
		Nqn:        nqn,
		Nsid:       "1",
		DevicePath: devicePath,
		Enabled:    true,
	}.Json())
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("namespace create: %w", err)
	}

	cmd = append(sh.cmds.nvmetKeeperCmd, "subsys", "attach", nqn, p.nvmetPort)
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("subsys attach: %w", err)
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

func (sh *share) Publish(ctx context.Context, shareData string, mi mounts.MounterNodeInfo) (*shares.PublishContext, error) {
	c, err := parseShareData(shareData)
	if err != nil {
		return nil, fmt.Errorf("parse share data: %w", err)
	}

	nvmeInfo, err := mntrnvme.ParseNvmeInfo(mi)
	if err != nil {
		return nil, fmt.Errorf("parse mounter info as nvme: %w", err)
	}

	cmd := append(sh.cmds.nvmetKeeperCmd, "host", "create", nvmet.Host{
		Nqn:      nvmeInfo.HostNqn,
		NeedAuth: false,
	}.Json())
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("host create: %w", err)
	}

	cmd = append(sh.cmds.nvmetKeeperCmd, "subsys", "allow", c.Nqn, nvmeInfo.HostNqn)
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("subsys allow: %w", err)
	}

	pc := map[string]string{}
	err = mntrnvme.InjectPublishContext(pc, mntrnvme.NvmePublishContext{
		Nqn:       c.Nqn,
		Port:      c.Port,
		Transport: c.Transport,
		IoQueues:  c.IoQueues,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateVolumeContext: %w", err)
	}

	return &shares.PublishContext{
		Public: pc,
		Local: map[string]string{
			"host_nqn": nvmeInfo.HostNqn,
		},
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, shareData string, localCtx map[string]string) error {
	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}

	hostNqn := localCtx["host_nqn"]
	if hostNqn == "" {
		return fmt.Errorf("publish_context: public: field missing: host_nqn")
	}

	cmd := append(sh.cmds.nvmetKeeperCmd, "subsys", "disallow", c.Nqn, hostNqn)
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("subsys disallow: %w", err)
	}

	return nil
}

func (sh *share) UnpublishAll(ctx context.Context, shareData string) error {
	return fmt.Errorf("UnpublishAll is not implemented")
}

func (sh *share) DeleteShare(ctx context.Context, shareData string) error {
	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}

	cmd := append(sh.cmds.nvmetKeeperCmd, "subsys", "detach", c.Nqn, c.Port)
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("subsys detach: %w", err)
	}

	cmd = append(sh.cmds.nvmetKeeperCmd, "subsys", "delete", c.Nqn)
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("subsys delete: %w", err)
	}

	return nil
}

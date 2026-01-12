package shrtniscsi

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/shares/truenas"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/strzfszvol"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mntriscsiadm"
)

type share struct {
	gp   globalParams
	cmds truenas.Commands
}

func New(commands map[string][]string, globalParams map[string]string) (shares.ShareHelper, error) {
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	cmds, err := truenas.ParseCommands(commands)
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
		LocalVolume:  []string{storage.VolDevice},
		OpaqueVolume: []string{"zfs-zvol"},
	}
}

func (sh *share) CreateShare(ctx context.Context, req shares.CreateShareRequest) (string, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return "", fmt.Errorf("parse params: %w", err)
	}
	_ = p

	zfsVol, ok := req.Volume.(*strzfszvol.Volume)
	if !ok {
		return "", fmt.Errorf("storage volume is not zfs zvol: %T", req.Volume)
	}

	iqn := fmt.Sprintf("%s:%s", sh.gp.iqnBase, req.ShareId)

	snip := newSnippets(ctx, sh.cmds.Midclt)

	err = snip.setupTarget(tnIscsiRequest{
		extentName: uuid.New().String(),
		iqn:        iqn,
		zvol:       zfsVol.Data.Path,
	})
	if err != nil {
		return "", err
	}

	c := shareContext{
		Iqn:  iqn,
		Zvol: zfsVol.Data.Path,
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

	iscsiInfo, err := mntriscsiadm.ParseIscsiInfo(mi)
	if err != nil {
		return nil, fmt.Errorf("parse mounter info as iscsi: %w", err)
	}

	snip := newSnippets(ctx, sh.cmds.Midclt)

	err = snip.attachGroup(c.Iqn, iscsiInfo.HostIqn)
	if err != nil {
		return nil, fmt.Errorf("attach iscsi group: %w", err)
	}

	pc := map[string]string{}
	err = mntriscsiadm.InjectPublishContext(pc, mntriscsiadm.IscsiPublishContext{
		Iqn:       c.Iqn,
		Port:      "3260",
		Transport: "tcp",
		Chap:      mntriscsiadm.ChapNone,
	})
	if err != nil {
		return nil, fmt.Errorf("create publish context: %w", err)
	}
	return &shares.PublishContext{
		Public: pc,
		Local: map[string]string{
			"host_iqn": iscsiInfo.HostIqn,
		},
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, shareData string, localCtx map[string]string) error {
	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}

	hostIqn := localCtx["host_iqn"]
	if hostIqn == "" {
		return fmt.Errorf("local publish context: missing field 'host_iqn'")
	}
	snip := newSnippets(ctx, sh.cmds.Midclt)

	err = snip.detachGroup(c.Iqn, hostIqn)
	if err != nil {
		return err
	}
	return nil
}

func (sh *share) UnpublishAll(ctx context.Context, shareData string) error {
	return nil
}

func (sh *share) DeleteShare(ctx context.Context, shareData string) error {
	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}

	snip := newSnippets(ctx, sh.cmds.Midclt)

	err = snip.removeIscsiTarget(c.Iqn)
	if err != nil {
		return err
	}
	err = snip.removeIscsiExtents(c.Zvol)
	if err != nil {
		return err
	}

	return nil
}

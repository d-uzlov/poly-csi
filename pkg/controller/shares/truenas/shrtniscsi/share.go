package shrtniscsi

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/shares/truenas"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/strzfszvol"
)

type share struct {
	gp   globalParams
	cmds truenas.Commands
}

func New(commands map[string][]string, globalParams map[string]string) (shares.ShareManager, error) {
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
		UnpublishAll: false,
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
		zvol:       zfsVol.Path,
	})
	if err != nil {
		return "", err
	}

	c := shareContext{
		Iqn:  iqn,
		Zvol: zfsVol.Path,
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

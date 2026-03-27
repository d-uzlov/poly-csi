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

	snip := newSnippets(sh.tr)

	err = snip.setupTarget(ctx, tnIscsiRequest{
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

	snip := newSnippets(sh.tr)

	err = snip.removeIscsiTarget(ctx, c.Iqn)
	if err != nil {
		return err
	}
	err = snip.removeIscsiExtents(ctx, c.Zvol)
	if err != nil {
		return err
	}

	return nil
}

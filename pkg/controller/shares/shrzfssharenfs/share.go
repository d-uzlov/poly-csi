package shrzfssharenfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/strzfsdataset"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrnfs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type share struct {
	tr   transport.Transport
	cmds commands
}

func New(tr transport.Transport, commands map[string][]string, globalParams map[string]string) (shares.ShareManager, error) {
	cmds, err := parseCommands(commands)
	if err != nil {
		return nil, fmt.Errorf("parse commands: %w", err)
	}
	return &share{
		tr:   tr,
		cmds: cmds,
	}, nil
}

func (sh *share) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: true,
		LocalVolume:  []string{storage.VolFolder},
		OpaqueVolume: []string{"zfs-dataset"},
	}
}

func (sh *share) CreateShare(ctx context.Context, req shares.CreateShareRequest) (string, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return "", fmt.Errorf("parse params: %w", err)
	}

	zfsVol, ok := req.Volume.(*strzfsdataset.Volume)
	if !ok {
		return "", fmt.Errorf("storage volume is not a zfs dataset: %T", req.Volume)
	}

	mountPath, err := req.Volume.GetLocalPath(ctx)
	if err != nil {
		return "", fmt.Errorf("get local path: %w", err)
	}

	snip := zfs.NewSnippets(sh.tr, sh.cmds.zfsCmd)

	err = snip.SetShareNfs(ctx, zfsVol.Path, p.ShareOptions)
	if err != nil {
		return "", err
	}

	c := shareContext{
		Dataset: zfsVol.Path,
		Folder:  mountPath,
	}
	err = c.validate()
	if err != nil {
		return "", fmt.Errorf("validate share context: %w", err)
	}

	return c.serialize(), nil
}

func (sh *share) Publish(ctx context.Context, shareData string, mi connectors.ConnectorNodeInfo) (*shares.PublishContext, error) {
	c, err := parseShareData(shareData)
	if err != nil {
		return nil, fmt.Errorf("parse share data: %w", err)
	}

	return &shares.PublishContext{
		Public: cntrnfs.PublicContext{
			Path: c.Folder,
		},
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, shareData string, serialized string) error {
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

	snip := zfs.NewSnippets(sh.tr, sh.cmds.zfsCmd)

	return snip.SetShareNfs(ctx, c.Dataset, "off")
}

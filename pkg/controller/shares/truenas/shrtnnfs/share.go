package shrtnnfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/shares/truenas"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mntrnfs"
)

type share struct {
	cmds truenas.Commands
}

func New(commands map[string][]string, globalParams map[string]string) (shares.ShareManager, error) {
	cmds, err := truenas.ParseCommands(commands)
	if err != nil {
		return nil, fmt.Errorf("parse commands: %w", err)
	}
	return &share{
		cmds: cmds,
	}, nil
}

func (sh *share) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: true,
		LocalVolume:  []string{storage.VolFolder},
		OpaqueVolume: nil,
	}
}

func (sh *share) CreateShare(ctx context.Context, req shares.CreateShareRequest) (string, error) {
	snip := newNfsSnippets(ctx, sh.cmds.Midclt)

	mountPath, err := req.Volume.GetLocalPath(ctx)
	if err != nil {
		return "", fmt.Errorf("get local path: %w", err)
	}

	err = snip.configureNfsShare(mountPath)
	if err != nil {
		return "", err
	}

	c := shareContext{
		Folder: mountPath,
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

	pc := map[string]string{}
	err = mntrnfs.InjectPublishContext(pc, mntrnfs.NfsMountContext{
		Path: c.Folder,
	})
	if err != nil {
		return nil, fmt.Errorf("inject nfs publish context: %w", err)
	}
	return &shares.PublishContext{
		Public: pc,
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

	snip := newNfsSnippets(ctx, sh.cmds.Midclt)

	return snip.removeNfsShare(c.Folder)
}

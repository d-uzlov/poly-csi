package shrnfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mntrnfs"
)

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

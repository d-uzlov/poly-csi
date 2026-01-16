package strbtrfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/utils/btrfs"
)

type Volume struct {
	containerPath string
	snip          btrfs.Snippets
}

var _ storage.Volume = &Volume{}

func newVolume(path string, snip btrfs.Snippets) *Volume {
	return &Volume{
		containerPath: path,
		snip:          snip,
	}
}

func parseVolume(serialized string, snip btrfs.Snippets) (*Volume, error) {
	return &Volume{
		containerPath: serialized,
		snip:          snip,
	}, nil
}

func (v *Volume) Serialize() (string, error) {
	return v.containerPath, nil
}

func (v *Volume) GetLocalPath(ctx context.Context) (string, error) {
	return v.getMainVolume(), nil
}

func (v *Volume) getMainVolume() string {
	return v.containerPath + "/volume"
}

func (v *Volume) Create(ctx context.Context) error {
	_, err := v.snip.SubvolumeCreate(ctx, v.getMainVolume())
	if err != nil {
		return fmt.Errorf("create subvol: %w", err)
	}
	return nil
}

func (v *Volume) setQuota(ctx context.Context, size int64) error {
	err := v.snip.SetQuota(ctx, v.getMainVolume(), size)
	if err != nil {
		return fmt.Errorf("set subvol quota: %w", err)
	}
	return nil
}

func (v *Volume) Delete(ctx context.Context) error {
	_, err := v.snip.SubvolumeDelete(ctx, v.getMainVolume())
	if err != nil {
		return err
	}
	return nil
}

func (v *Volume) GetSize(ctx context.Context) (int64, error) {
	// TODO uncomment proper returns  after updating to a newer btrfs version
	info, err := v.snip.GetQuotaInfo(ctx, v.getMainVolume())
	if err != nil {
		return 0, nil
		// return 0, err
	}
	size := info.GetQuota()
	if size <= 0 {
		return 0, nil
		// return 0, fmt.Errorf("quota not set")
	}
	return size, nil
}

func (v *Volume) Expand(ctx context.Context, newSize int64) (int64, error) {
	info, err := v.snip.GetQuotaInfo(ctx, v.getMainVolume())
	if err != nil {
		return 0, err
	}
	currentSize := info.GetQuota()
	if currentSize > 0 && newSize <= currentSize {
		return currentSize, nil
	}

	err = v.snip.SetQuota(ctx, v.getMainVolume(), newSize)
	if err != nil {
		return 0, err
	}

	return newSize, nil
}

func (v *Volume) MakeNativeSnapshot(ctx context.Context, name string) (storage.Snapshot, error) {
	snapshotPath := v.containerPath + "/snap-" + name
	exists, err := v.snip.SubvolumeTest(ctx, snapshotPath)
	if exists {
		return newSnapshot(snapshotPath, v.snip), nil
	}

	err = v.snip.SubvolumeSnapshot(ctx, v.getMainVolume(), snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("create btrfs snapshot '%s': %w", snapshotPath, err)
	}

	return newSnapshot(snapshotPath, v.snip), nil
}

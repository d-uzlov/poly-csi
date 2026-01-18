package strzfszvol

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type Volume struct {
	Path string `json:"path"`
	snip zfs.Snippets
}

var _ storage.Volume = &Volume{}

func newVolume(path string, snip zfs.Snippets) *Volume {
	return &Volume{
		Path: path,
		snip: snip,
	}
}

func parseVolume(encoded string, snip zfs.Snippets) (*Volume, error) {
	var result Volume
	err := json.Unmarshal([]byte(encoded), &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	if result.Path == "" {
		return nil, fmt.Errorf("zfs dataset path is empty")
	}
	result.snip = snip
	return &result, nil
}

func (v *Volume) Serialize() (string, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (v *Volume) GetLocalPath(ctx context.Context) (string, error) {
	return "/dev/zvol/" + v.Path, nil
}

func (v *Volume) Delete(ctx context.Context) error {
	_, err := v.snip.DestroyDataset(ctx, v.Path)
	if err != nil {
		return err
	}
	return nil
}

func (v *Volume) GetSize(ctx context.Context) (int64, error) {
	info, err := v.snip.GetProperties(ctx, v.Path)
	if err != nil {
		return 0, err
	}
	if info == nil {
		return 0, fmt.Errorf("dataset not found")
	}

	return info.GetSize(), nil
}

func (v *Volume) Expand(ctx context.Context, newSize int64) (int64, error) {
	info, err := v.snip.GetProperties(ctx, v.Path)
	if err != nil {
		return 0, err
	}
	if info == nil {
		return 0, fmt.Errorf("dataset not found")
	}

	currentSize := info.GetSize()
	if newSize <= currentSize {
		return currentSize, nil
	}

	err = v.snip.ConfigureDataset(ctx, v.Path, zfs.DatasetConfig{
		Quota:          newSize,
		RefQuota:       newSize,
		Reservation:    0,
		RefReservation: 0,
	})
	if err != nil {
		return 0, err
	}

	return newSize, nil
}

func (v *Volume) MakeSnapshot(ctx context.Context, name string) (storage.Snapshot, error) {
	snapshotName := fmt.Sprintf("%s@%s", v.Path, name)

	_, err := v.snip.CreateSnapshot(ctx, snapshotName)
	if err != nil {
		return nil, fmt.Errorf("create zfs snapshot '%s': %w", snapshotName, err)
	}

	return newSnapshot(snapshotName, v.snip), nil
}

package strzfsdataset

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/zfsdata"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type Volume struct {
	Data      zfsdata.ZfsData
	snip      zfs.Snippets
	mountPath string
}

var _ storage.Volume = &Volume{}

func newVolume(data zfsdata.ZfsData, snip zfs.Snippets) *Volume {
	return &Volume{
		Data: data,
		snip: snip,
	}
}

func (v *Volume) GetLocalPath(ctx context.Context) (string, error) {
	if v.mountPath != "" {
		return v.mountPath, nil
	}
	datasetInfo, err := v.snip.GetDataset(ctx, v.Data.Path)
	if err != nil {
		return "", fmt.Errorf("checking dataset: %w", err)
	}
	if datasetInfo == nil {
		return "", fmt.Errorf("dataset is not present in underlying storage")
	}
	if datasetInfo.IsZvol() {
		return "", fmt.Errorf("expected dataset but found zvol at path %s", v.Data.Path)
	}
	v.mountPath = datasetInfo.Properties.MountPoint.Value

	return v.mountPath, nil
}

func (v *Volume) Delete(ctx context.Context) error {
	_, err := v.snip.DestroyDataset(ctx, v.Data.Path)
	if err != nil {
		return err
	}
	return nil
}

func (v *Volume) GetSize(ctx context.Context) (int64, error) {
	datasetInfo, err := v.snip.GetDataset(ctx, v.Data.Path)
	if err != nil {
		return 0, err
	}
	if datasetInfo == nil {
		return 0, fmt.Errorf("dataset not found")
	}

	return datasetInfo.GetSize(), nil
}

func (v *Volume) Expand(ctx context.Context, newSize int64) (int64, error) {
	datasetInfo, err := v.snip.GetDataset(ctx, v.Data.Path)
	if err != nil {
		return 0, err
	}
	if datasetInfo == nil {
		return 0, fmt.Errorf("dataset not found")
	}

	currentSize := datasetInfo.GetSize()
	if newSize <= currentSize {
		return currentSize, nil
	}

	err = v.snip.ConfigureDataset(ctx, v.Data.Path, zfs.DatasetConfig{
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

func (v *Volume) Serialize() (string, error) {
	return v.Data.Serialize()
}

func (v *Volume) MakeNativeSnapshot(ctx context.Context, name string) (storage.Snapshot, error) {
	snapshotName := fmt.Sprintf("%s@%s", v.Data.Path, name)

	_, err := v.snip.CreateSnapshot(ctx, snapshotName)
	if err != nil {
		return nil, fmt.Errorf("create zfs snapshot '%s': %w", snapshotName, err)
	}

	// it's ok if mountPath is not filled
	return newSnapshot(snapshotName, v.snip, v.mountPath), nil
}

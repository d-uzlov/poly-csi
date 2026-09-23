package strzfszvol

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
		return nil, fmt.Errorf("path is empty")
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

func (v *Volume) CreateState(ctx context.Context, size int64) error {
	_, err := v.snip.CreateZvol(ctx, v.Path, zfs.ZvolConfig{
		Size: size,
	})
	if err != nil {
		return fmt.Errorf("create zvol: %w", err)
	}

	return nil
}

func (v *Volume) ReadState(ctx context.Context) (*storage.VolumeState, error) {
	info, err := v.snip.GetProperties(ctx, v.Path)
	if err != nil {
		return nil, fmt.Errorf("checking dataset: %w", err)
	}
	if info == nil {
		return nil, nil
	}
	if !info.IsZvol() {
		return nil, fmt.Errorf("expected zvol but found dataset at path '%s'", v.Path)
	}
	blockSize, err := info.Properties.VolBlockSize.GetInt()
	if err != nil {
		return nil, fmt.Errorf("read volblocksize: %w", err)
	}
	result := storage.VolumeState{
		FsPath:           "/dev/zvol/" + v.Path,
		Size:             info.GetSize(),
		RMWSize:          blockSize,
		AtomicSize:       blockSize,
		// TODO use vdev geometry here?
		OptimalWriteSize: blockSize,
	}

	return &result, nil
}

func (v *Volume) DeleteState(ctx context.Context) error {
	_, err := v.snip.DestroyDataset(ctx, v.Path)
	if err != nil {
		return err
	}
	return nil
}

func (v *Volume) SetSize(ctx context.Context, newSize int64) error {
	return v.snip.ConfigureZvol(ctx, v.Path, zfs.ZvolConfig{
		Size: newSize,
	})
}

func (v *Volume) MakeSnapshot(ctx context.Context, name string) (storage.Snapshot, error) {
	snapshotName := fmt.Sprintf("%s@%s", v.Path, name)

	_, err := v.snip.CreateSnapshot(ctx, snapshotName)
	if err != nil {
		return nil, fmt.Errorf("create zfs snapshot '%s': %w", snapshotName, err)
	}

	return newSnapshot(snapshotName, v.snip), nil
}

func (v *Volume) ReceiveSnapshot(ctx context.Context, sourceFile string, timeout time.Duration) error {
	targetSnapshot := fmt.Sprintf("%s@init", v.Path)
	defer v.snip.DestroySnapshot(ctx, targetSnapshot)

	err := v.snip.Receive(ctx, targetSnapshot, sourceFile, timeout)
	if err != nil {
		return fmt.Errorf("zfs receive: %w", err)
	}
	return nil
}

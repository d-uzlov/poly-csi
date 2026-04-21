package strbtrfs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/btrfs"
)

type Volume struct {
	ContainerPath string `json:"container_path"`

	snip btrfs.Snippets
	tr   transport.Transport
}

var _ storage.Volume = &Volume{}

func newVolume(path string, snip btrfs.Snippets, tr transport.Transport) *Volume {
	return &Volume{
		ContainerPath: path,
		snip:          snip,
		tr:            tr,
	}
}

func parseVolume(encoded string, snip btrfs.Snippets) (*Volume, error) {
	var result Volume
	err := json.Unmarshal([]byte(encoded), &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	if result.ContainerPath == "" {
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
	err := v.tr.SudoCmds().Mkdir(ctx, v.ContainerPath, false)
	if err != nil {
		return fmt.Errorf("create volume container folder: %w", err)
	}

	err = v.createMainVolume(ctx)
	if err != nil {
		return fmt.Errorf("createMainVolume: %w", err)
	}

	err = v.setQuota(ctx, size)
	if err != nil {
		return fmt.Errorf("set quota: %w", err)
	}

	return nil
}

func (v *Volume) ReadState(ctx context.Context) (*storage.VolumeState, error) {
	size, err := v.GetSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("read size: %w", err)
	}
	result := storage.VolumeState{
		FsPath: v.getMainVolume(),
		Size:   size,
	}

	return &result, nil
}

func (v *Volume) getMainVolume() string {
	return v.ContainerPath + "/volume"
}

func (v *Volume) createMainVolume(ctx context.Context) error {
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

func (v *Volume) DeleteState(ctx context.Context) error {
	_, err := v.snip.SubvolumeDelete(ctx, v.getMainVolume())
	if err != nil {
		return fmt.Errorf("delete main volume: %w", err)
	}
	list, err := v.tr.Cmds().Ls(ctx, v.ContainerPath)
	if err != nil {
		return fmt.Errorf("list volume container files: %w", err)
	}
	if len(list) == 0 {
		err := v.tr.Cmds().RmDir(ctx, v.ContainerPath)
		if err != nil {
			return fmt.Errorf("delete empty volume container: %w", err)
		}
	}
	return nil
}

func (v *Volume) GetSize(ctx context.Context) (int64, error) {
	// TODO btrfs crashes when trying to get quota info in json format
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

func (v *Volume) Expand(ctx context.Context, newSize int64) error {
	return v.snip.SetQuota(ctx, v.getMainVolume(), newSize)
}

func (v *Volume) MakeSnapshot(ctx context.Context, name string) (storage.Snapshot, error) {
	snapshotPath := v.ContainerPath + "/snap-" + name
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

func (v *Volume) ReceiveSnapshot(ctx context.Context, sourceFile string, timeout time.Duration) error {
	err := v.tr.SudoCmds().Mkdir(ctx, v.ContainerPath, false)
	if err != nil {
		return fmt.Errorf("create volume container folder: %w", err)
	}

	volumeName, err := v.snip.Receive(ctx, v.ContainerPath, sourceFile, timeout)
	if err != nil {
		return fmt.Errorf("btrfs receive: %w", err)
	}
	receivedVolPath := v.ContainerPath + "/" + volumeName

	if receivedVolPath == v.getMainVolume() {
		// TODO should this be an error?
		return nil
	}

	defer func() {
		_, deferErr := v.snip.SubvolumeDelete(ctx, receivedVolPath)
		if deferErr != nil {
			logs.FromCtx(ctx).Error("failed to cleanup received btrfs volume", "path", receivedVolPath, "error", deferErr)
		}
	}()
	err = v.snip.SubvolumeClone(ctx, receivedVolPath, v.getMainVolume())
	if err != nil {
		return fmt.Errorf("clone received subvol: %w", err)
	}

	return nil
}

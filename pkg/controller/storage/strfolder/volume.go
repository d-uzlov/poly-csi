package strfolder

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
)

type Volume struct {
	tr transport.Transport

	Path string `json:"path"`
}

var _ storage.Volume = &Volume{}

func newVolume(tr transport.Transport, path string) *Volume {
	return &Volume{
		tr:   tr,
		Path: path,
	}
}

func parseVolume(tr transport.Transport, encoded string) (*Volume, error) {
	var result Volume
	err := json.Unmarshal([]byte(encoded), &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	if result.Path == "" {
		return nil, fmt.Errorf("path is empty")
	}
	result.tr = tr
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
	err := v.tr.SudoCmds().Mkdir(ctx, v.Path, true)
	if err != nil {
		return fmt.Errorf("create volume folder: %w", err)
	}

	return nil
}

func (v *Volume) ReadState(ctx context.Context) (*storage.VolumeState, error) {
	sr, err := v.tr.Cmds().Stat(ctx, v.Path)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	if !sr.FileExists {
		return nil, nil
	}
	result := storage.VolumeState{
		FsPath: v.Path,
		Size:   0,
	}

	return &result, nil
}

func (v *Volume) DeleteState(ctx context.Context) error {
	err := v.tr.SudoCmds().RmDirRecursive(ctx, v.Path)
	if err != nil {
		return fmt.Errorf("remove %s: %w", v.Path, err)
	}
	return nil
}

func (v *Volume) Expand(ctx context.Context, newSize int64) error {
	return nil
}

func (v *Volume) MakeSnapshot(ctx context.Context, name string) (storage.Snapshot, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (v *Volume) ReceiveSnapshot(ctx context.Context, sourceFile string, timeout time.Duration) error {
	return fmt.Errorf("unimplemented")
}

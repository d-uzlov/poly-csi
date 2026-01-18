package strfolder

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
)

type Volume struct {
	Path string `json:"path"`
}

var _ storage.Volume = &Volume{}

func newVolume(path string) *Volume {
	return &Volume{
		Path: path,
	}
}

func parseVolume(encoded string) (*Volume, error) {
	var result Volume
	err := json.Unmarshal([]byte(encoded), &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	if result.Path == "" {
		return nil, fmt.Errorf("path is empty")
	}
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
	return v.Path, nil
}

func (v *Volume) Delete(ctx context.Context) error {
	err := transport.FromContext(ctx).SudoCmds().RmDirRecursive(ctx, v.Path)
	if err != nil {
		return fmt.Errorf("remove %s: %w", v.Path, err)
	}
	return nil
}

func (v *Volume) GetSize(ctx context.Context) (int64, error) {
	return 0, nil
}

func (v *Volume) Expand(ctx context.Context, newSize int64) (int64, error) {
	return 0, nil
}

func (v *Volume) MakeSnapshot(ctx context.Context, name string) (storage.Snapshot, error) {
	return nil, fmt.Errorf("unimplemented")
}

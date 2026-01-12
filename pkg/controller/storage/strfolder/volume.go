package strfolder

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
)

type Volume struct {
	path string
}

var _ storage.Volume = &Volume{}

func (v *Volume) GetLocalPath(ctx context.Context) (string, error) {
	return v.path, nil
}

func (v *Volume) Delete(ctx context.Context) error {
	err := deleteFolder(ctx, v.path)
	if err != nil {
		return err
	}
	return nil
}

func (v *Volume) GetSize(ctx context.Context) (int64, error) {
	return 0, nil
}

func (v *Volume) Expand(ctx context.Context, newSize int64) (int64, error) {
	return 0, nil
}

func (v *Volume) Serialize() (string, error) {
	return v.path, nil
}

func (v *Volume) MakeNativeSnapshot(ctx context.Context, name string) (storage.Snapshot, error) {
	return nil, fmt.Errorf("unimplemented")
}

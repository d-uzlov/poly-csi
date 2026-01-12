package strzfsdataset

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type Snapshot struct {
	FullName string `json:"full_name"`
	snip     zfs.Snippets
}

var _ storage.Snapshot = &Snapshot{}

func newSnapshot(fullName string, snip zfs.Snippets) *Snapshot {
	return &Snapshot{
		FullName: fullName,
		snip:     snip,
	}
}

func (s *Snapshot) GetCapabilities() storage.Capabilities {
	return storage.Capabilities{
		LocalVolume:    storage.VolFolder,
		OpaqueVolume:   "zfs-dataset",
		NativeSnapshot: true,
		LocalSend:      "zfs-dataset",
	}
}

func (s *Snapshot) Send(ctx context.Context, targetFile string, timeout time.Duration) error {
	err := s.snip.Send(ctx, s.FullName, targetFile, timeout)
	if err != nil {
		return fmt.Errorf("zfs send: %w", err)
	}

	return nil
}

func (s *Snapshot) Delete(ctx context.Context) error {
	_, err := s.snip.DestroySnapshot(ctx, s.FullName)
	return err
}

func (s *Snapshot) Serialize() (string, error) {
	bytes, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

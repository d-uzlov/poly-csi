package strbtrfs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/utils/btrfs"
)

type Snapshot struct {
	Path string `json:"full_name"`
	snip btrfs.Snippets
}

var _ storage.Snapshot = &Snapshot{}

func newSnapshot(path string, snip btrfs.Snippets) *Snapshot {
	return &Snapshot{
		Path: path,
		snip: snip,
	}
}

func parseSnapshot(serialized string, snip btrfs.Snippets) (*Snapshot, error) {
	var result Snapshot
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	if result.Path == "" {
		return nil, fmt.Errorf("path is empty")
	}
	result.snip = snip

	return &result, nil
}

func (s *Snapshot) Serialize() (string, error) {
	bytes, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (s *Snapshot) GetCapabilities() storage.Capabilities {
	return storage.Capabilities{
		LocalVolume:    storage.VolFolder,
		OpaqueVolume:   "btrfs-subvolume",
		NativeSnapshot: true,
		LocalSend:      "btrfs-subvolume",
	}
}

func (s *Snapshot) Send(ctx context.Context, targetFile string, timeout time.Duration) error {
	err := s.snip.Send(ctx, s.Path, targetFile, timeout)
	if err != nil {
		return fmt.Errorf("btrfs send: %w", err)
	}

	return nil
}

func (s *Snapshot) Delete(ctx context.Context) error {
	_, err := s.snip.SubvolumeDelete(ctx, s.Path)
	return err
}

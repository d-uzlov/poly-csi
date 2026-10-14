package strbtrfs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/filevol"
	"meoe.io/poly-csi/pkg/utils/btrfs"
)

type Snapshot struct {
	SubvolPath string `json:"full_name"`
	snip       btrfs.Snippets
}

var _ filevol.Snapshot = &Snapshot{}

func newSnapshot(path string, snip btrfs.Snippets) *Snapshot {
	return &Snapshot{
		SubvolPath: path,
		snip:       snip,
	}
}

func parseSnapshot(serialized json.RawMessage, snip btrfs.Snippets) (*Snapshot, error) {
	var result Snapshot
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	if result.SubvolPath == "" {
		return nil, fmt.Errorf("path is empty")
	}
	result.snip = snip

	return &result, nil
}

func (s *Snapshot) Serialize() (json.RawMessage, error) {
	return json.Marshal(s)
}

func (s *Snapshot) GetLocalPath(ctx context.Context) (path string, release func(), err error) {
	return s.SubvolPath, func() {}, nil
}

func (s *Snapshot) Send(ctx context.Context, targetFile string, timeout time.Duration) error {
	err := s.snip.Send(ctx, s.SubvolPath, targetFile, timeout)
	if err != nil {
		return fmt.Errorf("btrfs send: %w", err)
	}

	return nil
}

func (s *Snapshot) Delete(ctx context.Context) error {
	_, err := s.snip.SubvolumeDelete(ctx, s.SubvolPath)
	return err
}

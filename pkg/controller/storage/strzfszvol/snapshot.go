package strzfszvol

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type Snapshot struct {
	FullName   string
	parentName string
	snip       zfs.Snippets
}

var _ storage.Snapshot = &Snapshot{}

func newSnapshot(fullName string, snip zfs.Snippets) *Snapshot {
	c := strings.Split(fullName, "@")
	if len(c) != 2 {
		panic("snapshot name is not in the format dataset@snap-name: " + fullName)
	}
	if c[0] == "" || c[1] == "" {
		panic("snapshot name is not in the format dataset@snap-name: " + fullName)
	}
	return &Snapshot{
		FullName:   fullName,
		parentName: c[0],
		snip:       snip,
	}
}

func (s *Snapshot) Serialize() (string, error) {
	bytes, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (s *Snapshot) GetLocalPath(ctx context.Context) (path string, release func(), err error) {
	err = s.snip.PropSet(ctx, s.parentName, "snapdev", "visible")
	if err != nil {
		return "", nil, fmt.Errorf("set snapdev=visible: %w", err)
	}
	path = fmt.Sprintf("/dev/zvol/%s", s.FullName)
	release = func() {
		err = s.snip.PropInherit(ctx, s.parentName, "snapdev")
		if err != nil {
			logs.FromCtx(ctx).Error("failed to configure 'inherit snapdev' on cleanup")
		}
	}
	return path, release, nil
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

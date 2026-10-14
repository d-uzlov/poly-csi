package strzfsdataset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"meoe.io/poly-csi/pkg/controller/filevol"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type Snapshot struct {
	FullName   string `json:"full_name"`
	parentName string
	snapName   string
	mountPath  string
	snip       zfs.Snippets
}

var _ filevol.Snapshot = &Snapshot{}

func newSnapshot(fullName string, snip zfs.Snippets, mountPath string) *Snapshot {
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
		snapName:   c[1],
		mountPath:  mountPath,
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
	if s.mountPath != "" {
		return s.mountPath, nil, nil
	}
	info, err := s.snip.GetProperties(ctx, s.parentName)
	if err != nil {
		return "", nil, fmt.Errorf("checking dataset: %w", err)
	}
	if info == nil {
		return "", nil, fmt.Errorf("dataset is not present in underlying storage")
	}
	if info.IsZvol() {
		return "", nil, fmt.Errorf("expected dataset but found zvol at path %s", s.parentName)
	}
	s.mountPath = fmt.Sprintf("%s/.zfs/snapshot/%s", info.Properties.MountPoint.Value, s.snapName)
	return s.mountPath, func() {}, nil
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

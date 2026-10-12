package fsnfs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/filecon"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

type state struct {
	tr transport.Transport `json:"-"`

	NfsPath string `json:"nfsPath"`
}

var _ filecon.Handle = &state{}

func parseState(serialized string) (*state, error) {
	var result state
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *state) Prepare(ctx context.Context, req filecon.PrepareRequest) error {
	return nil
}

func (s *state) Mount(ctx context.Context, req filecon.MountRequest) error {
	mi, err := fsutils.New(s.tr).EnsureMounted(ctx, s.NfsPath, req.StagingPath, "nfs", req.Flags)
	if err != nil {
		return err
	}
	if mi == nil {
		return fmt.Errorf("mount succeeded but fs info is nil")
	}
	if !strings.HasPrefix(mi.FsType, "nfs") {
		return fmt.Errorf("fs is already mounted with different fs type: %s != nfs, at mountpoint %s", mi.FsType, mi.MountPoint)
	}
	if mi.Source != s.NfsPath {
		return fmt.Errorf("already mounted from different source: %s != %s: mounted at %s", mi.Source, s.NfsPath, mi.MountPoint)
	}

	return nil
}

func (s *state) Unmount(ctx context.Context, path string) error {
	return fsutils.New(s.tr).Unmount(ctx, path)
}

func (s *state) Disconnect(ctx context.Context) error {
	// unmount is managed externally
	return nil
}

func (s *state) Expand(ctx context.Context) error {
	// nfs volumes don't need expanding
	return nil
}

func (s *state) CheckHealth(ctx context.Context) ([]filecon.HealthMessage, error) {
	return nil, nil
}

func (s *state) Serialize() (string, error) {
	res, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func (s *state) Validate() error {
	if s.tr == nil {
		return fmt.Errorf("missing transport")
	}
	if s.NfsPath == "" {
		return fmt.Errorf("missing nfs share path")
	}
	return nil
}

package cntrcifs

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

type state struct {
	tr transport.Transport `json:"-"`

	SharePath string `json:"sharePath"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

var _ connectors.Handle = &state{}

func parseState(serialized string) (*state, error) {
	var result state
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *state) ConnectBlock(ctx context.Context) error {
	return fmt.Errorf("nvme: ConnectBlock is not implemented")
}

func (s *state) ConnectFolder(ctx context.Context, req connectors.ConnectFolderRequest) error {
	if req.FsType != "cifs" {
		return fmt.Errorf("expected cifs fs type but got %s", req.FsType)
	}

	flags := req.MountFlags
	if req.MountGroup != "" {
		flags = append(flags, fmt.Sprintf("gid=%s", req.MountGroup))
	}
	flags = append(flags, fmt.Sprintf("username=%s", s.Username), fmt.Sprintf("password=%s", s.Password))

	mi, err := fsutils.New(s.tr).EnsureMounted(ctx, s.SharePath, req.FsMountPath, req.FsType, flags)
	if err != nil {
		return err
	}
	if mi == nil {
		return fmt.Errorf("mount succeeded but fs info is nil")
	}
	if mi.FsType != "cifs" {
		return fmt.Errorf("fs is already mounted with different fs type: %s != cifs, at mountpoint %s", mi.FsType, mi.MountPoint)
	}
	if mi.Source != s.SharePath {
		return fmt.Errorf("already mounted from different source: %s != %s: mounted at %s", mi.Source, s.SharePath, mi.MountPoint)
	}

	return nil
}

func (s *state) GetDevices(ctx context.Context, index int) ([]string, error) {
	return nil, fmt.Errorf("cifs does not create block devices")
}

func (s *state) Disconnect(ctx context.Context) error {
	// unmount is managed externally
	return nil
}

func (s *state) Expand(ctx context.Context) error {
	// cifs volumes don't need expanding
	return nil
}

func (s *state) CheckHealth(ctx context.Context) ([]connectors.HealthMessage, error) {
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
		return fmt.Errorf("transport is not set")
	}
	if s.SharePath == "" {
		return fmt.Errorf("missing SharePath")
	}
	return nil
}

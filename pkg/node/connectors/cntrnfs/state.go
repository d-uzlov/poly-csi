package cntrnfs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

type state struct {
	tr transport.Transport `json:"-"`

	NfsPath    string   `json:"nfsPath"`
}

var _ connectors.ConnectionHandle = &state{}

func parseState(serialized string) (*state, error) {
	var result state
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *state) ConnectBlock(ctx context.Context) (err error) {
	return fmt.Errorf("nvme: ConnectBlock is not implemented")
}

func (s *state) ConnectFolder(ctx context.Context, req connectors.ConnectFolderRequest) error {
	mi, err := fsutils.New(s.tr).EnsureMounted(ctx, s.NfsPath, req.FsMountPath, req.FsType, req.MountFlags)
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

func (s *state) GetDevices(ctx context.Context, index int) ([]string, error) {
	return nil, fmt.Errorf("nfs does not create block devices")
}

func (s *state) Disconnect(ctx context.Context) error {
	// unmount is managed externally
	return nil
}

func (s *state) Expand(ctx context.Context) error {
	// nfs volumes don't need expanding
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
		return fmt.Errorf("missing transport")
	}
	if s.NfsPath == "" {
		return fmt.Errorf("missing nfs share path")
	}
	return nil
}

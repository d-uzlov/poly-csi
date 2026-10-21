package fsext4

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/fsspec"
	"meoe.io/poly-csi/pkg/node/filecon"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

type state struct {
	tr transport.Transport `json:"-"`

	Data    fsspec.Ext4Data   `json:"data"`
	Devices map[string]string `json:"devices"`
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
	s.Devices = req.Devices
	u := fsutils.New(s.tr)
	snippets := NewSnippets(s.tr, s.Data, req.Devices)
	blkid, err := u.GetBlkid(ctx, snippets.mainDevice)
	if err != nil {
		return fmt.Errorf("blkid '%s': %w", snippets.mainDevice, err)
	}
	if blkid == nil {
		err = snippets.Format(ctx)
		if err != nil {
			return fmt.Errorf("format: %w", err)
		}
	}

	err = snippets.Mount(ctx, req.StagingPath, req.Flags)
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}

	return nil
}

func (s *state) Mount(ctx context.Context, req filecon.MountRequest) error {
	err := fsutils.New(s.tr).EnsureBindMount(ctx, req.StagingPath, req.TargetPath)
	if err != nil {
		return err
	}

	if req.FsGroup != "" {
		err = s.tr.Cmds().Chown(ctx, req.TargetPath, "root", req.FsGroup, false)
		if err != nil {
			return fmt.Errorf("chown fs root: %w", err)
		}
	}
	return nil
}

func (s *state) Unmount(ctx context.Context, path string) error {
	return fsutils.New(s.tr).Unmount(ctx, path)
}

func (s *state) Disconnect(ctx context.Context) error {
	return nil
}

func (s *state) Expand(ctx context.Context) error {
	snippets := NewSnippets(s.tr, s.Data, s.Devices)
	return snippets.Resize(ctx)
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
	err := s.Data.Validate()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	return nil
}

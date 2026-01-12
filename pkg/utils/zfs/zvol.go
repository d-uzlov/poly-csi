package zfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type ZvolConfig struct {
	Size int64
}

func (s *Snippets) CreateZvol(ctx context.Context, path string, config ZvolConfig) (alreadyExisted bool, err error) {
	cmd := append(s.zfsCmd,
		"create", path,
		"-V", fmt.Sprintf("%d", config.Size),
	)
	recovered, err := transport.RelaxedRun(ctx, cmd, "dataset already exists")
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	return recovered, nil
}

func (s *Snippets) ConfigureZvol(ctx context.Context, path string, config ZvolConfig) error {
	cmd := append(s.zfsCmd,
		"set",
		fmt.Sprintf("volsize=%d", config.Size),
		path,
	)
	err := transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

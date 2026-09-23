package zfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type ZvolConfig struct {
	Size         int64
	Volblocksize int64
	Reservation  int64
}

func (s *Snippets) CreateZvol(ctx context.Context, path string, config ZvolConfig) (alreadyExisted bool, err error) {
	cmd := append(zfsCmd,
		"create", path,
		"-V", fmt.Sprintf("%d", config.Size),
		"-o", fmt.Sprintf("refreservation=%d", config.Reservation),
	)
	if config.Volblocksize != 0 {
		cmd = append(cmd, "-o", fmt.Sprintf("volblocksize=%d", config.Volblocksize))
	}
	recovered, err := transport.RelaxedRun(ctx, s.tr, cmd, "dataset already exists", transport.WithSudo(true))
	if err != nil {
		return false, fmt.Errorf("zfs create: %w", err)
	}
	return recovered, nil
}

func (s *Snippets) ConfigureZvol(ctx context.Context, path string, config ZvolConfig) error {
	cmd := append(zfsCmd,
		"set",
		fmt.Sprintf("volsize=%d", config.Size),
		path,
	)
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("zfs set size: %w", err)
	}
	return nil
}

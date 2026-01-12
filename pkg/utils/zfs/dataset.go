package zfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type DatasetConfig struct {
	Quota          int64
	RefQuota       int64
	Reservation    int64
	RefReservation int64
}

func (s *Snippets) CreateDataset(ctx context.Context, path string, config DatasetConfig) (alreadyExisted bool, err error) {
	cmd := append(s.zfsCmd,
		"create", path,
		"-o", fmt.Sprintf("quota=%d", config.Quota),
		"-o", fmt.Sprintf("refquota=%d", config.RefQuota),
		"-o", fmt.Sprintf("reservation=%d", config.Reservation),
		"-o", fmt.Sprintf("refreservation=%d", config.RefReservation),
	)
	recovered, err := transport.RelaxedRun(ctx, cmd, "dataset already exists")
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	return recovered, nil
}

func (s *Snippets) ConfigureDataset(ctx context.Context, path string, config DatasetConfig) error {
	cmd := append(s.zfsCmd,
		"set",
		fmt.Sprintf("quota=%d", config.Quota),
		fmt.Sprintf("refquota=%d", config.RefQuota),
		fmt.Sprintf("reservation=%d", config.Reservation),
		fmt.Sprintf("refreservation=%d", config.RefReservation),
		path,
	)
	err := transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (s *Snippets) SetShareNfs(ctx context.Context, path string, nfsOptions string) error {
	cmd := append(s.zfsCmd, "set", "sharenfs="+nfsOptions, path)
	err := transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

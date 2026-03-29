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
	cmd := append(zfsCmd,
		"create", path,
		"-o", fmt.Sprintf("quota=%d", config.Quota),
		"-o", fmt.Sprintf("refquota=%d", config.RefQuota),
		"-o", fmt.Sprintf("reservation=%d", config.Reservation),
		"-o", fmt.Sprintf("refreservation=%d", config.RefReservation),
	)
	recovered, err := transport.RelaxedRun(ctx, s.tr, cmd, "dataset already exists", transport.WithSudo(true))
	if err != nil {
		return false, fmt.Errorf("zfs create: %w", err)
	}
	return recovered, nil
}

func (s *Snippets) ConfigureDataset(ctx context.Context, path string, config DatasetConfig) error {
	cmd := append(zfsCmd,
		"set",
		fmt.Sprintf("quota=%d", config.Quota),
		fmt.Sprintf("refquota=%d", config.RefQuota),
		fmt.Sprintf("reservation=%d", config.Reservation),
		fmt.Sprintf("refreservation=%d", config.RefReservation),
		path,
	)
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("zfs set size: %w", err)
	}
	return nil
}

func (s *Snippets) SetShareNfs(ctx context.Context, path string, nfsOptions string) error {
	cmd := append(zfsCmd, "set", "sharenfs="+nfsOptions, path)
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("zfs set sharenfs: %w", err)
	}
	return nil
}

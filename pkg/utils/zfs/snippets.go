package zfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type Snippets struct {
	tr     transport.Transport
	zfsCmd []string
}

func NewSnippets(tr transport.Transport, zfsCmd []string) Snippets {
	return Snippets{
		tr:     tr,
		zfsCmd: zfsCmd,
	}
}

func (s *Snippets) DestroyDataset(ctx context.Context, path string) (recovered bool, err error) {
	cmd := append(s.zfsCmd, "destroy", path)
	recovered, err = transport.RelaxedRun(ctx, s.tr, cmd, "dataset does not exist")
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	return recovered, nil
}

func (s *Snippets) PropSet(ctx context.Context, path string, prop string, value string) error {
	cmd := append(s.zfsCmd, "set", fmt.Sprintf("%s=%s", prop, value), path)
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (s *Snippets) PropInherit(ctx context.Context, path string, prop string) error {
	cmd := append(s.zfsCmd, "inherit", prop, path)
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

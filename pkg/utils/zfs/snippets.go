package zfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

var zfsCmd = []string{"zfs"}

type Snippets struct {
	tr     transport.Transport
}

func NewSnippets(tr transport.Transport) Snippets {
	return Snippets{
		tr:     tr,
	}
}

func (s *Snippets) DestroyDataset(ctx context.Context, path string) (recovered bool, err error) {
	cmd := append(zfsCmd, "destroy", path)
	recovered, err = transport.RelaxedRun(ctx, s.tr, cmd, "dataset does not exist", transport.WithSudo(true))
	if err != nil {
		return false, fmt.Errorf("zfs destroy: %w", err)
	}
	return recovered, nil
}

func (s *Snippets) PropSet(ctx context.Context, path string, prop string, value string) error {
	cmd := append(zfsCmd, "set", fmt.Sprintf("%s=%s", prop, value), path)
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("zfs set %s: %w", prop, err)
	}
	return nil
}

func (s *Snippets) PropInherit(ctx context.Context, path string, prop string) error {
	cmd := append(zfsCmd, "inherit", prop, path)
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("Zfs inherit %s: %w", prop, err)
	}
	return nil
}

package zfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type Snippets struct {
	zfsCmd []string
}

func NewSnippets(zfsCmd []string) Snippets {
	return Snippets{
		zfsCmd: zfsCmd,
	}
}

func (s *Snippets) DestroyDataset(ctx context.Context, path string) (recovered bool, err error) {
	cmd := append(s.zfsCmd, "destroy", path)
	recovered, err = transport.RelaxedRun(ctx, cmd, "dataset does not exist")
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	return recovered, nil
}

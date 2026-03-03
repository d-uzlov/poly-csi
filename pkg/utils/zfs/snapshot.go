package zfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

func (s *Snippets) CreateSnapshot(ctx context.Context, name string) (alreadyExisted bool, err error) {
	cmd := append(s.zfsCmd,
		"snapshot",
		name,
	)
	recovered, err := transport.RelaxedRun(ctx, s.tr, cmd, "dataset already exists")
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	return recovered, nil
}

func (s *Snippets) DestroySnapshot(ctx context.Context, name string) (recovered bool, err error) {
	cmd := append(s.zfsCmd, "destroy", name)
	recovered, err = transport.RelaxedRun(ctx, s.tr, cmd, "could not find any snapshots to destroy; check snapshot names")
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	return recovered, nil
}

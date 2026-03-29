package lvm

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

var lvmCmd = []string{"lvm"}

type Snippets struct {
	tr transport.Transport
}

func NewSnippets(tr transport.Transport) Snippets {
	return Snippets{
		tr: tr,
	}
}

func (s *Snippets) MakeLvPath(vgName string, lvName string) string {
	return fmt.Sprintf("/dev/%s/%s", vgName, lvName)
}

func (s *Snippets) LvCreate(ctx context.Context, vgName string, lvName string, size int64) (alreadyExists bool, err error) {
	cmd := append(lvmCmd, "lvcreate", "--yes", fmt.Sprintf("--size=%dB", size), vgName, "--name", lvName)
	alreadyExists, err = transport.RelaxedRun(ctx, s.tr, cmd, "already exists in volume group", transport.WithSudo(true))
	if err != nil {
		return false, err
	}
	return alreadyExists, nil
}

func (s *Snippets) LvDelete(ctx context.Context, vgName string, lvName string) (notFound bool, err error) {
	cmd := append(lvmCmd, "lvremove", "-y", s.MakeLvPath(vgName, lvName))
	notFound, err = transport.RelaxedRun(ctx, s.tr, cmd, "Failed to find logical volume", transport.WithSudo(true))
	if err != nil {
		return false, err
	}
	return notFound, nil
}

func (s *Snippets) LvSnapshot(ctx context.Context, vgName string, lvName string, snapName string, size int64) (alreadyExists bool, err error) {
	cmd := append(lvmCmd, "lvcreate", "--snapshot", s.MakeLvPath(vgName, lvName), fmt.Sprintf("--size=%dB", size), "--name", snapName, "--permission=r")
	alreadyExists, err = transport.RelaxedRun(ctx, s.tr, cmd, "already exists in volume group", transport.WithSudo(true))
	if err != nil {
		return false, err
	}
	return alreadyExists, nil
}

func (s *Snippets) LvSetSize(ctx context.Context, vgName string, lvName string, newSize int64) error {
	cmd := append(lvmCmd, "lvresize", fmt.Sprintf("--size=%dB", newSize), s.MakeLvPath(vgName, lvName))
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "matches existing size", transport.WithSudo(true))
	if err != nil {
		return err
	}
	return nil
}

func (s *Snippets) LvSetActive(ctx context.Context, vgName string, lvName string, value bool) error {
	argVal := "n"
	if value {
		argVal = "y"
	}
	cmd := append(lvmCmd, "lvchange", "-y", "--activate", argVal, s.MakeLvPath(vgName, lvName))
	err := transport.CheckedRun(ctx, s.tr, cmd, transport.WithSudo(true))
	if err != nil {
		return err
	}
	return nil
}

package btrfs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"meoe.io/poly-csi/pkg/transport"
)

type Snippets struct {
	btrfsCmd []string
}

type BtrfsJsonResult struct {
	Header struct {
		Version string `json:"version"`
	} `json:"__header"`
	QuotaEntries []QuotaEntry `json:"qgroup-show"`
	FilesystemDf []QuotaEntry `json:"filesystem-df"`
}

func NewSnippets(zfsCmd []string) Snippets {
	return Snippets{
		btrfsCmd: zfsCmd,
	}
}

func (s *Snippets) SubvolumeTest(ctx context.Context, path string) (exists bool, err error) {
	cmd := append(s.btrfsCmd, "subvolume", "show", path)
	recovered, err := transport.RelaxedRun(ctx, cmd, "Not a Btrfs subvolume")
	if err != nil {
		return false, err
	}
	if recovered {
		return false, nil
	}
	return true, nil
}

func (s *Snippets) CreateSubvolume(ctx context.Context, path string) (recovered bool, err error) {
	cmd := append(s.btrfsCmd, "subvolume", "create", path)
	recovered, err = transport.RelaxedRun(ctx, cmd, "target path already exists")
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	return recovered, nil
}

func (s *Snippets) DestroySubvolume(ctx context.Context, path string) (recovered bool, err error) {
	cmd := append(s.btrfsCmd, "subvolume", "delete", path)
	recovered, err = transport.RelaxedRun(ctx, cmd, "No such file or directory")
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	return recovered, nil
}

func (s *Snippets) SnapshotSubvolume(ctx context.Context, source string, dest string) error {
	cmd := append(s.btrfsCmd, "subvolume", "snapshot", "-r", source, dest)
	err := transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (s *Snippets) Send(ctx context.Context, sourceSnapshot string, targetFile string, timeout time.Duration) error {
	cmd := append(s.btrfsCmd,
		"send",
		sourceSnapshot,
	)
	tr := transport.FromContext(ctx)
	runResult, err := tr.RunCommand(ctx, cmd, transport.WithStdoutFile(targetFile), transport.WithTimeout(timeout))
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

func (s *Snippets) Receive(ctx context.Context, targetParent string, sourceFile string, timeout time.Duration) (string, error) {
	cmd := append(s.btrfsCmd,
		"receive",
		targetParent,
	)
	tr := transport.FromContext(ctx)
	runResult, err := tr.RunCommand(ctx, cmd, transport.WithStdinFile(sourceFile), transport.WithTimeout(timeout))
	if err != nil {
		return "", fmt.Errorf("%s: %w", cmd, err)
	}
	if runResult.ExitCode != 0 {
		return "", transport.FormatRunError(runResult)
	}

	stdout := string(runResult.Stdout)
	stdout = strings.TrimSpace(stdout)
	subvol, found := strings.CutPrefix(stdout, "At subvol ")
	if !found {
		return "", fmt.Errorf("btrfs receive did not print subvol name")
	}
	return subvol, nil
}

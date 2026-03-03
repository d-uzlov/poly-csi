package btrfs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"meoe.io/poly-csi/pkg/transport"
)

type Snippets struct {
	tr       transport.Transport
	btrfsCmd []string
}

type BtrfsJsonResult struct {
	Header struct {
		Version string `json:"version"`
	} `json:"__header"`
	QuotaEntries []QuotaEntry `json:"qgroup-show"`
	FilesystemDf []QuotaEntry `json:"filesystem-df"`
}

func NewSnippets(tr transport.Transport, btrfsCmd []string) Snippets {
	return Snippets{
		tr:       tr,
		btrfsCmd: btrfsCmd,
	}
}

func (s *Snippets) SubvolumeTest(ctx context.Context, path string) (exists bool, err error) {
	cmd := append(s.btrfsCmd, "subvolume", "show", path)
	recovered, err := transport.RelaxedRun(ctx, s.tr, cmd, "Not a Btrfs subvolume")
	if err != nil {
		return false, err
	}
	if recovered {
		return false, nil
	}
	return true, nil
}

func (s *Snippets) SubvolumeCreate(ctx context.Context, path string) (alreadyExists bool, err error) {
	cmd := append(s.btrfsCmd, "subvolume", "create", path)
	alreadyExists, err = transport.RelaxedRun(ctx, s.tr, cmd, "target path already exists")
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	return alreadyExists, nil
}

func (s *Snippets) SubvolumeDelete(ctx context.Context, path string) (doesNotExist bool, err error) {
	cmd := append(s.btrfsCmd, "subvolume", "delete", path)
	doesNotExist, err = transport.RelaxedRun(ctx, s.tr, cmd, "No such file or directory")
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	return doesNotExist, nil
}

func (s *Snippets) SubvolumeSnapshot(ctx context.Context, source string, dest string) error {
	cmd := append(s.btrfsCmd, "subvolume", "snapshot", "-r", source, dest)
	err := transport.CheckedRun(ctx, s.tr, cmd)
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
	runResult, err := s.tr.RunCommand(ctx, cmd, transport.WithStdoutFile(targetFile), transport.WithTimeout(timeout))
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
	runResult, err := s.tr.RunCommand(ctx, cmd, transport.WithStdinFile(sourceFile), transport.WithTimeout(timeout))
	if err != nil {
		return "", fmt.Errorf("%s: %w", cmd, err)
	}
	if runResult.ExitCode != 0 {
		return "", transport.FormatRunError(runResult)
	}

	stderr := strings.TrimSpace(string(runResult.Stderr))
	subvol, found := strings.CutPrefix(stderr, "At subvol ")
	if !found {
		stdout := strings.TrimSpace(string(runResult.Stdout))
		if stderr != "" && stdout != "" {
			return "", fmt.Errorf("btrfs receive did not print subvol name: stdout='%s', stderr='%s'", stdout, stderr)
		}
		if stderr != "" {
			return "", fmt.Errorf("btrfs receive did not print subvol name: stderr='%s'", stderr)
		}
		if stdout != "" {
			return "", fmt.Errorf("btrfs receive did not print subvol name: stdout='%s'", stdout)
		}
		return "", transport.FormatRunError(runResult)
	}
	return subvol, nil
}

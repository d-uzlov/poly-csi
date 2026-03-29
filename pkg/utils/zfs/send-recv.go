package zfs

import (
	"context"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/transport"
)

func (s *Snippets) Send(ctx context.Context, sourceSnapshot string, targetFile string, timeout time.Duration) error {
	cmd := append(zfsCmd,
		"send",
		"--large-block",
		sourceSnapshot,
	)
	runResult, err := s.tr.RunCommand(ctx, cmd, transport.WithStdoutFile(targetFile), transport.WithTimeout(timeout))
	if err != nil {
		return fmt.Errorf("zfs send: %w", err)
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

func (s *Snippets) Receive(ctx context.Context, targetSnapshot string, sourceFile string, timeout time.Duration) error {
	cmd := append(zfsCmd,
		"receive",
		targetSnapshot,
	)
	runResult, err := s.tr.RunCommand(ctx, cmd, transport.WithSudo(true), transport.WithStdinFile(sourceFile), transport.WithTimeout(timeout))
	if err != nil {
		return fmt.Errorf("zfs receive: %w", err)
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

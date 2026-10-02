package ublk

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
)

func (s *Snippets) CreateForeground(ctx context.Context, configDir string, cr CreateParams, extra ExtraParams, report func(*DeviceInfo), handleLogLine func(string)) error {
	cmd := []string{cr.GetForegroundBinary(), "add"}
	cmd = append(cmd, cr.MakeParams()...)
	if extra == nil {
		return fmt.Errorf("ExtraParams is nil")
	}
	cmd = append(cmd, extra.MakeParams()...)

	return s.runForeground(ctx, configDir, cmd, report, handleLogLine)
}

func (s *Snippets) RecoverForeground(ctx context.Context, configDir string, cr CreateParams, report func(*DeviceInfo), handleLogLine func(string)) error {
	cmd := []string{cr.GetForegroundBinary(), "recover", "-n", cr.DevNum}
	return s.runForeground(ctx, configDir, cmd, report, handleLogLine)
}

// runForeground launches ublk process in foreground and tries to parse device info
// if the device info is found, it calls report(info), and then waits for ublk to finish
// if ublk did not print device info, it calls report(nil) and returns an error
func (s *Snippets) runForeground(ctx context.Context, configDir string, cmd []string, report func(*DeviceInfo), handleLogLine func(string)) error {
	parser := ListParser{}

	const (
		stateInitial = iota
		stateParsing
		stateError
		stateBackground
	)
	state := stateInitial

	var stdoutLines []string

	// we put stdout lines into stdoutLines array
	// if we detect device info, we dump stdoutLines into logs and start parsing device info
	// if any error happens, all further log lines will go into stdoutLines and then converted to return value

	rr, err := s.tr.RunCommand(ctx, cmd, transport.WithEnv("UBLKSRV_RUN_DIR", configDir), transport.WithoutTimeout(), transport.WithStreamingStdoutLines(func(line string) {
		if state == stateInitial && parser.IsDeviceInfoStart(line) {
			for _, v := range stdoutLines {
				handleLogLine(v)
			}
			stdoutLines = nil
			state = stateParsing
		}
		switch state {
		case stateInitial:
			stdoutLines = append(stdoutLines, line)
		case stateParsing:
			info, err := parser.PushLine(line)
			if err != nil {
				logs.FromCtx(ctx).Warn("detected parsing error", "line", line, "error", err)
				report(nil)
				state = stateError
				stdoutLines = append(stdoutLines, line)
				return
			}

			if info == nil {
				return
			}
			report(info)
			state = stateBackground
		case stateError:
			stdoutLines = append(stdoutLines, line)
		case stateBackground:
			handleLogLine(line)
		}
	}))
	if err != nil {
		return err
	}
	rr.Stdout = []byte(strings.Join(stdoutLines, "\n"))
	if rr.ExitCode != 0 {
		return transport.FormatRunError(rr)
	}

	return nil
}

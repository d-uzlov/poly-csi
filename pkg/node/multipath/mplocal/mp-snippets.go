package mplocal

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
)

type snippets struct {
	tr           transport.Transport
	multipathCmd []string
}

func newSnippets(tr transport.Transport, multipathCmd []string) *snippets {
	return &snippets{
		tr:           tr,
		multipathCmd: multipathCmd,
	}
}

func (s *snippets) mpCommand(command string) []string {
	// multipathd does not support "-k" and "command" as different arguments,
	// it required that "command" is concatenated to "-k"
	return append(s.multipathCmd, "-k"+command)
}

func (s *snippets) addDevices(ctx context.Context, devices []string) error {
	if len(devices) == 0 {
		return fmt.Errorf("no devices as input")
	}
	for _, v := range devices {
		cmd := s.mpCommand(fmt.Sprintf("add path %s", v))
		err := transport.CheckedRun(ctx, s.tr, cmd)
		if err != nil {
			return fmt.Errorf("add device to multipath: %w", err)
		}
	}
	return nil
}

func (s *snippets) status(ctx context.Context) error {
	cmd := s.mpCommand("show status")
	recovered, err := transport.RelaxedRun(ctx, s.tr, cmd, "failed to connect to multipathd")
	if err != nil {
		return err
	}
	if recovered {
		return fmt.Errorf("failed to connect to multipathd")
	}
	return nil
}

func (s *snippets) removeMapDevices(ctx context.Context, mapName string) error {
	cmd := s.mpCommand("show paths raw format %d/%m")
	stdout, err := transport.CheckedRunStdout(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("list multipath devices: %w", err)
	}
	for v := range strings.Lines(stdout) {
		device, name, ok := strings.Cut(v, "/")
		if !ok {
			logs.FromCtx(ctx).Warn("invalid multipath show paths line format: %s", v)
		}
		if name == mapName {
			cmd = s.mpCommand(fmt.Sprintf("del path %s", device))
			_, err := transport.RelaxedRun(ctx, s.tr, cmd, "device not found")
			if err != nil {
				return fmt.Errorf("delete device from multipath: %w", err)
			}
		}
	}

	return nil
}

func (s *snippets) findMapName(ctx context.Context, devices []string) (string, error) {
	cmd := s.mpCommand("show paths raw format %d/%m")
	stdout, err := transport.CheckedRunStdout(ctx, s.tr, cmd)
	if err != nil {
		return "", fmt.Errorf("list multipath devices: %w", err)
	}
	for v := range strings.Lines(stdout) {
		device, name, ok := strings.Cut(strings.TrimSpace(v), "/")
		if !ok {
			logs.FromCtx(ctx).Warn("show paths: invalid line format: %s", v)
		}
		if device == devices[0] {
			if strings.Contains(name, "/") {
				return "", fmt.Errorf("invalid multipath map name: %s", name)
			}
			return name, nil
		}
	}

	return "", fmt.Errorf("could not find device in the multipath device list")
}

func (s *snippets) resizeMap(ctx context.Context, mapName string) error {
	cmd := s.mpCommand(fmt.Sprintf("resize map %s", mapName))
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("request resize: %w", err)
	}
	return nil
}

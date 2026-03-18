package mplocal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/transport"
)

type localMp struct {
	tr        transport.Transport
	config    Config
	devFolder string
}

type Config struct {
	MapperFolder  string   `yaml:"mapperFolder"`
	MultipathdCmd []string `yaml:"multipathdCmd"`
}

func (c *Config) Validate() error {
	if c.MapperFolder == "" {
		return fmt.Errorf("missing parameter mapperFolder")
	}
	if len(c.MultipathdCmd) == 0 {
		return fmt.Errorf("missing parameter multipathdCmd")
	}
	return nil
}

func NewLocalMp(tr transport.Transport, config Config, devFolder string) multipath.Multipath {
	return &localMp{
		tr:        tr,
		config:    config,
		devFolder: devFolder,
	}
}

func (l *localMp) mpCommand(command string) []string {
	// multipathd does not support "-k" and "command" as different arguments,
	// it required that "command" is concatenated to "-k"
	return append(l.config.MultipathdCmd, "-k"+command)
}

func (l *localMp) Ready(ctx context.Context) bool {
	cmd := l.mpCommand("show status")
	recovered, err := transport.RelaxedRun(ctx, l.tr, cmd, "failed to connect to multipathd")
	if err != nil {
		err = fmt.Errorf("multipathd -k'show status': %w", err)
		slogctx.FromCtx(ctx).Error("multipath Status check failed", "error", err.Error())
		return false
	}
	if recovered {
		return false
	}
	return true
}

func (l *localMp) Setup(ctx context.Context, devices []string) (*multipath.MultipathContext, error) {
	var shortDevices []string
	for _, v := range devices {
		shortDevices = append(shortDevices, strings.TrimPrefix(strings.TrimPrefix(v, l.devFolder), "/"))
	}
	name, err := l.enableMultipath(ctx, shortDevices)
	if err != nil {
		return nil, fmt.Errorf("setupViaMultipathd: %w", err)
	}

	deviceLink := l.config.MapperFolder + "/" + name

	encoded, err := json.Marshal(Data{
		MapName: name,
	})
	if err != nil {
		return nil, fmt.Errorf("encode context data: %w", err)
	}

	return &multipath.MultipathContext{
		Device:         deviceLink,
		SerializedData: string(encoded),
	}, nil
}

func (l *localMp) enableMultipath(ctx context.Context, devices []string) (string, error) {
	if len(devices) == 0 {
		return "", fmt.Errorf("no devices as input")
	}

	for _, v := range devices {
		cmd := l.mpCommand(fmt.Sprintf("add path %s", v))
		err := transport.CheckedRun(ctx, l.tr, cmd)
		if err != nil {
			return "", fmt.Errorf("add device to multipath: %w", err)
		}
	}

	cmd := l.mpCommand("show paths raw format %d/%m")
	stdout, err := transport.CheckedRunStdout(ctx, l.tr, cmd)
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

func (l *localMp) Destroy(ctx context.Context, serializedData string) error {
	data, err := parseData(serializedData)
	if err != nil {
		return fmt.Errorf("parse serialized data: %w", err)
	}

	cmd := l.mpCommand("show paths raw format %d/%m")
	stdout, err := transport.CheckedRunStdout(ctx, l.tr, cmd)
	if err != nil {
		return fmt.Errorf("list multipath devices: %w", err)
	}
	for v := range strings.Lines(stdout) {
		device, name, ok := strings.Cut(v, "/")
		if !ok {
			logs.FromCtx(ctx).Warn("invalid multipath show paths line format: %s", v)
		}
		if name == data.MapName {
			cmd = l.mpCommand(fmt.Sprintf("del path %s", device))
			_, err := transport.RelaxedRun(ctx, l.tr, cmd, "device not found")
			if err != nil {
				return fmt.Errorf("delete device from multipath: %w", err)
			}
		}
	}

	// when you delete all paths, multipath delete map automatically
	// cmd = l.mpCommand(fmt.Sprintf("del map %s", data.MapName))
	// err = transport.CheckedRun(ctx, l.tr, cmd)
	// if err != nil {
	// 	return fmt.Errorf("add device to multipath: %w", err)
	// }
	return nil
}

func (l *localMp) Expand(ctx context.Context, serializedData string) error {
	data, err := parseData(serializedData)
	if err != nil {
		return fmt.Errorf("parse serialized data: %w", err)
	}

	cmd := l.mpCommand(fmt.Sprintf("resize map %s", data.MapName))
	err = transport.CheckedRun(ctx, l.tr, cmd)
	if err != nil {
		return fmt.Errorf("request resize: %w", err)
	}

	return nil
}

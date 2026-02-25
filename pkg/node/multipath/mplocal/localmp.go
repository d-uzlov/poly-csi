package mplocal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/transport"
)

type localMp struct {
	config Config
}

type Config struct{}

func NewLocalMp(config Config) multipath.Multipath {
	return &localMp{config: config}
}

func (l *localMp) Ready(ctx context.Context) bool {
	// multipathd does not support "-k value" so we need to concat it into "-kValue"
	cmd := []string{"multipathd", "-k" + "show status"}
	recovered, err := transport.RelaxedRun(ctx, cmd, "failed to connect to multipathd")
	if err != nil {
		err = fmt.Errorf("multipathd -k'show status': %w", err)
		slogctx.FromCtx(ctx).Error("multipath Status check failed", "error", err)
		return false
	}
	if recovered {
		return false
	}
	return true
}

func (l *localMp) Setup(ctx context.Context, devices []string) (*multipath.MultipathContext, error) {
	wwid := ""
	for _, v := range devices {
		cmd := []string{"multipath", "-a", v}
		stdout, err := transport.CheckedRunStdout(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("%s (add device to config): %w", cmd, err)
		}
		devWwid, err := parseMpAdd(stdout)
		if err != nil {
			return nil, fmt.Errorf("parseMpAdd: %w", err)
		}
		if wwid == "" {
			wwid = devWwid
		} else if wwid != devWwid {
			return nil, fmt.Errorf("devices in list has different wwid values: %s (%s) != %s (%s)", wwid, devices[0], devWwid, v)
		}
	}
	cmd := []string{"multipath", "-r"}
	_, err := transport.CheckedRunStdout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}

	tr := transport.FromContext(ctx)

	deviceLink := "/dev/mapper/" + wwid
	// device mapper creates links in background, need to wait for it to appear
	err = transport.Retry(time.Millisecond*100, time.Second, func() error {
		stat, err := tr.Cmds().Stat(ctx, deviceLink)
		if err != nil {
			// runtime errors should be self-explanatory
			return err
		}
		if !stat.FileExists {
			return fmt.Errorf("link does not exist: %s", deviceLink)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wait for device file: %w", err)
	}

	encoded, err := json.Marshal(Data{
		Wwid: wwid,
	})
	if err != nil {
		return nil, fmt.Errorf("encode context data: %w", err)
	}

	return &multipath.MultipathContext{
		Device:         deviceLink,
		SerializedData: string(encoded),
	}, nil
}

func (l *localMp) Destroy(ctx context.Context, serializedData string) error {
	data, err := parseData(serializedData)
	if err != nil {
		return fmt.Errorf("parse serialized data: %w", err)
	}

	cmd := []string{"multipath", "-f", data.Wwid}
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s (disable DM device): %w", cmd, err)
	}
	cmd = []string{"multipath", "-w", data.Wwid}
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s (remove wwid from config): %w", cmd, err)
	}
	return nil
}

func parseMpAdd(stdout string) (string, error) {
	// example:
	// wwid '360014054975efecf0dd41ae980917107' added
	const prefix = "wwid '"
	const suffix = "' added"
	stdout = strings.TrimSpace(stdout)
	wwid, ok := strings.CutPrefix(stdout, prefix)
	if !ok {
		return "", fmt.Errorf("invalid multipath stdout format: expected to find \"%s\" prefix: %s", prefix, stdout)
	}
	wwid, ok = strings.CutSuffix(wwid, suffix)
	if !ok {
		return "", fmt.Errorf("invalid multipath stdout format: expected to find \"%s\" suffix: %s", suffix, stdout)
	}
	return wwid, nil
}

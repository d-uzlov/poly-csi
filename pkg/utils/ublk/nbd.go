package ublk

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type NbdTarget struct {
	DevSize    int64  `json:"dev_size"`
	ExportName string `json:"export_name"`
	Host       string `json:"host"`
	Port       string `json:"port"`
}

func (s *Snippets) CreateNbd(ctx context.Context, opt CreateNbd) (UblkDeviceInfo, error) {
	cmd := append(ublkCmd, "add",
		"--type", "nbd",
		"--user_recovery", "1",
		"--host", opt.Host,
		"--export_name", opt.ExportName,
	)
	if opt.Port != "" {
		cmd = append(cmd, "--port", opt.Port)
	}
	if opt.Zerocopy {
		cmd = append(cmd, "--zerocopy")
	}
	if opt.SendZeroCopy {
		cmd = append(cmd, "--send_zc", "1")
	}
	rr, err := s.tr.RunCommand(ctx, cmd)
	if err != nil {
		return UblkDeviceInfo{}, fmt.Errorf("ublk add: %w", err)
	}
	if rr.ExitCode != 0 {
		return UblkDeviceInfo{}, fmt.Errorf("ublk add: %w", transport.FormatRunError(rr))
	}

	info, err := ParseUblkOutput(string(rr.Stdout))
	if err != nil {
		return UblkDeviceInfo{}, fmt.Errorf("parse ublk add output: %w", err)
	}

	return info, nil
}

func (di UblkDeviceInfo) ParseNbd() (NbdTarget, error) {
	if di.Target == "" {
		return NbdTarget{}, fmt.Errorf("target info is empty")
	}
	var result NbdTarget
	err := json.Unmarshal([]byte(di.Target), &result)
	if err != nil {
		return NbdTarget{}, fmt.Errorf("parse target info '%s': %w", di.Target, err)
	}
	return result, nil
}

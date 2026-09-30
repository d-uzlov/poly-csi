package ublk

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

var ublkCmd = []string{"ublk"}

type Snippets struct {
	tr transport.Transport
}

func New(tr transport.Transport) Snippets {
	return Snippets{
		tr: tr,
	}
}

func (s *Snippets) Delete(ctx context.Context, devId string) error {
	cmd := append(ublkCmd, "del", "-n", devId)
	return transport.CheckedRun(ctx, s.tr, cmd)
}

func (s *Snippets) Recover(ctx context.Context, devId string) (UblkDeviceInfo, error) {
	cmd := append(ublkCmd, "recover", "-n", devId)
	rr, err := s.tr.RunCommand(ctx, cmd)
	if err != nil {
		return UblkDeviceInfo{}, fmt.Errorf("ublk recover: %w", err)
	}
	if rr.ExitCode != 0 {
		return UblkDeviceInfo{}, fmt.Errorf("ublk recover: %w", transport.FormatRunError(rr))
	}

	info, err := ParseUblkOutput(string(rr.Stdout))
	if err != nil {
		return UblkDeviceInfo{}, fmt.Errorf("parse ublk recover output: %w", err)
	}

	return info, nil
}

func (s *Snippets) CheckFeatures(ctx context.Context, opt CreateNbd) (Flags, error) {
	cmd := append(ublkCmd, "features")
	rr, err := s.tr.RunCommand(ctx, cmd)
	if err != nil {
		return Flags{}, fmt.Errorf("ublk features: %w", err)
	}
	if rr.ExitCode != 0 {
		return Flags{}, fmt.Errorf("ublk features: %w", transport.FormatRunError(rr))
	}

	firstLine := ""
	for v := range strings.Lines(string(rr.Stdout)) {
		firstLine = v
		break
	}
	if firstLine == "" {
		return Flags{}, fmt.Errorf("ublk features: empty output")
	}

	flagsString, ok := strings.CutPrefix(firstLine, "ublk_drv features:")
	if !ok {
		return Flags{}, fmt.Errorf("ublk features: invalid format: '%s'", firstLine)
	}

	return ParseFlagsString(flagsString)
}

type Flags struct {
	F_zero_copy          bool
	F_comp_in_task       bool
	F_get_data           bool
	F_user_recovery      bool
	F_recovery_reissue   bool
	F_unprivileged_dev   bool
	F_cmd_ioctl_encode   bool
	F_user_copy          bool
	F_zoned              bool
	F_recovery_fail_io   bool
	F_update_size        bool
	F_auto_zc            bool
	F_quiesce            bool
	F_per_io_daemon      bool
	F_buf_reg_off_daemon bool
	F_batch_io           bool
	F_integrity          bool
	F_safe_stop_dev      bool
	F_no_auto_part_scan  bool
	F_shmem_zc           bool
}

const (
	zero_copy = 1 << iota
	comp_in_task
	get_data
	user_recovery
	recovery_reissue
	unprivileged_dev
	cmd_ioctl_encode
	user_copy
	zoned
	recovery_fail_io
	update_size
	auto_zc
	quiesce
	per_io_daemon
	buf_reg_off_daemon
	batch_io
	integrity
	safe_stop_dev
	no_auto_part_scan
	shmem_zc
)

func ParseFlagsString(fs string) (Flags, error) {
	fs = strings.TrimSpace(fs)
	fs = strings.TrimPrefix(fs, "0x")
	flags, err := strconv.ParseUint(fs, 16, 64)
	if err != nil {
		return Flags{}, fmt.Errorf("failed to parse '%s' as hex: %w", fs, err)
	}

	return ParseFlags(flags), nil
}

func ParseFlags(flags uint64) Flags {
	return Flags{
		F_zero_copy:          (flags & zero_copy) != 0,
		F_comp_in_task:       (flags & comp_in_task) != 0,
		F_get_data:           (flags & get_data) != 0,
		F_user_recovery:      (flags & user_recovery) != 0,
		F_recovery_reissue:   (flags & recovery_reissue) != 0,
		F_unprivileged_dev:   (flags & unprivileged_dev) != 0,
		F_cmd_ioctl_encode:   (flags & cmd_ioctl_encode) != 0,
		F_user_copy:          (flags & user_copy) != 0,
		F_zoned:              (flags & zoned) != 0,
		F_recovery_fail_io:   (flags & recovery_fail_io) != 0,
		F_update_size:        (flags & update_size) != 0,
		F_auto_zc:            (flags & auto_zc) != 0,
		F_quiesce:            (flags & quiesce) != 0,
		F_per_io_daemon:      (flags & per_io_daemon) != 0,
		F_buf_reg_off_daemon: (flags & buf_reg_off_daemon) != 0,
		F_batch_io:           (flags & batch_io) != 0,
		F_integrity:          (flags & integrity) != 0,
		F_safe_stop_dev:      (flags & safe_stop_dev) != 0,
		F_no_auto_part_scan:  (flags & no_auto_part_scan) != 0,
		F_shmem_zc:           (flags & shmem_zc) != 0,
	}
}

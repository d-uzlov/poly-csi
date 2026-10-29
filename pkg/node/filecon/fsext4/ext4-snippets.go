package fsext4

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"meoe.io/poly-csi/pkg/api/fsspec"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

type snippets struct {
	tr            transport.Transport
	Data          fsspec.Ext4Data
	mainDevice    string
	journalDevice string
}

func NewSnippets(tr transport.Transport, Data fsspec.Ext4Data, Devices map[string]string) *snippets {
	return &snippets{
		tr:            tr,
		Data:          Data,
		mainDevice:    Devices[mainVolumeKey],
		journalDevice: Devices[journalVolumeKey],
	}
}

func (s *snippets) Format(ctx context.Context) error {
	cmd := []string{"mkfs.ext4"}

	cmd = append(cmd, "-b", fmt.Sprintf("%d", s.Data.Format.BlockSize))

	if s.Data.User.SeparateJournal {
		cmd = append(cmd, "-J", fmt.Sprintf("device=%s", s.journalDevice))
		journalCmd := []string{"mke2fs", "-O", "journal_dev"}
		journalCmd = append(journalCmd, "-b", fmt.Sprintf("%d", s.Data.Format.BlockSize))
		journalCmd = append(journalCmd, s.journalDevice)
		err := transport.CheckedRun(ctx, s.tr, journalCmd)
		if err != nil {
			return fmt.Errorf("journal format: %w", err)
		}
	}

	cmd = append(cmd, "-m", strconv.FormatFloat(s.Data.User.ReservedPercent, 'f', 2, 64))
	cmd = append(cmd, "-i", s.Data.User.BytesPerInode)

	var extended []string
	if s.Data.Format.SkipDiscard {
		extended = append(extended, "assume_storage_prezeroed=1", "nodiscard")
	}
	if s.Data.Format.Stride > 0 {
		extended = append(extended, fmt.Sprintf("stride=%d", s.Data.Format.Stride))
	}
	if s.Data.Format.Stripe > 0 {
		extended = append(extended, fmt.Sprintf("stripe_width=%d", s.Data.Format.Stripe))
	}
	if len(extended) != 0 {
		cmd = append(cmd, "-E", strings.Join(extended, ","))
	}

	// putting device path last because alpine utils don't support options after device path
	cmd = append(cmd, s.mainDevice)

	// TODO run this in background?
	err := transport.CheckedRun(ctx, s.tr, cmd, transport.WithTimeout(60*time.Second))
	if err != nil {
		return fmt.Errorf("run format: %w", err)
	}
	return nil
}

func (s *snippets) GetMountOptions() []string {
	var result []string
	if s.journalDevice != "" {
		result = append(result, fmt.Sprintf("journal_path=%s", s.journalDevice))
	}
	return result
}

func (s *snippets) Mount(ctx context.Context, targetPath string, opts []string) error {
	u := fsutils.New(s.tr)
	cmd := []string{"mount", "-t", "ext4"}
	for _, v := range opts {
		cmd = append(cmd, "-o", v)
	}
	if s.journalDevice != "" {
		opts = append(opts, fmt.Sprintf("journal_path=%s", s.journalDevice))
	}
	if s.Data.Format.Stripe != 0 {
		opts = append(opts, fmt.Sprintf("stripe=%d", s.Data.Format.Stripe))
	}

	_, err := u.EnsureMounted(ctx, s.mainDevice, targetPath, "ext4", opts)
	return err
}

func (s *snippets) Resize(ctx context.Context) error {
	// TODO add journal resize
	cmd := []string{"resize2fs", s.mainDevice}
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("resize2fs: %w", err)
	}
	return nil
}

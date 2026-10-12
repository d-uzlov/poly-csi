package fsxfs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"meoe.io/poly-csi/pkg/api/fsapi"
	"meoe.io/poly-csi/pkg/transport"
)

type snippets struct {
	tr            transport.Transport
	Data          fsapi.XfsData
	mainDevice    string
	journalDevice string
}

func NewSnippets(tr transport.Transport, Data fsapi.XfsData, Devices map[string]string) *snippets {
	return &snippets{
		tr:            tr,
		Data:          Data,
		mainDevice:    Devices[mainVolumeKey],
		journalDevice: Devices[journalVolumeKey],
	}
}

func (s *snippets) Format(ctx context.Context) error {
	cmd := []string{"mkfs.xfs"}

	cmd = append(cmd, "-b", fmt.Sprintf("size=%d", s.Data.Format.BlockSize))
	cmd = append(cmd, "-s", fmt.Sprintf("size=%d", s.Data.Format.SectorSize))

	if s.Data.Format.SkipDiscard {
		cmd = append(cmd, "-K")
	}

	data := []string{
		fmt.Sprintf("su=%d", s.Data.Format.StripeUnitBytes),
		fmt.Sprintf("sw=%d", s.Data.Format.StripeWidth),
	}

	if len(data) != 0 {
		cmd = append(cmd, "-d", strings.Join(data, ","))
	}

	journalArgs := []string{}
	if s.Data.User.SeparateJournal {
		journalArgs = append(journalArgs,
			"logdev="+s.journalDevice,
			fmt.Sprintf("su=%d", s.Data.Format.JournalStripeUnitBytes),
		)
	}
	if len(journalArgs) != 0 {
		cmd = append(cmd, "-l", strings.Join(journalArgs, ","))
	}

	cmd = append(cmd, s.mainDevice)

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
	// TODO
	cmd := []string{"mount", "-t", "ext4"}
	for _, v := range opts {
		cmd = append(cmd, "-o", v)
	}
	if s.Data.User.SeparateJournal {
		cmd = append(cmd, "-o", fmt.Sprintf("journal_path=%s", s.journalDevice))
	}

	// can't use --source and --target options because mount in busybox does not support them
	cmd = append(cmd, s.mainDevice, targetPath)

	err := transport.CheckedRun(ctx, s.tr, cmd, transport.WithSudo(true))
	if err != nil {
		return err
	}
	return nil
}

func (s *snippets) Resize(ctx context.Context) error {
	cmd := []string{"xfs_growfs", s.mainDevice}
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("xfs_growfs: %w", err)
	}
	return nil
}

package volumeinit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type Ext4 struct {
	BytesPerInode    string  `map:"bytesPerInode" json:"bytes_per_inode"`
	JournalSizeDenom float64 `map:"journalSizeDenom" json:"journal_size_denom"`

	SkipDiscard     bool   `map:"-" json:"skipDiscard"`
	BlockSize       int64  `map:"-" json:"blockSize"`
	ReservedPercent string `map:"-" json:"reservedPercent"`

	mainDevice    string `map:"-" json:"-"`
	journalDevice string `map:"-" json:"-"`
}

const ext4Prefix = "ext4"

func NewExt4() FilesystemHelper {
	return &Ext4{}
}

func (f *Ext4) Capabilities() Capabilities {
	return Capabilities{
		// 10 MiB
		MinFsSize:       10 * 1024 * 1024,
		ExternalJournal: true,
	}
}

func (f *Ext4) Configure(params map[string]string) error {
	err := serialmaps.Parse(params, ext4Prefix, f, serialmaps.WithRemove(true))
	if err != nil {
		return err
	}

	if f.BytesPerInode == "" {
		f.BytesPerInode = "65536"
	}

	if f.JournalSizeDenom == 0.0 {
		f.JournalSizeDenom = 70
	}
	if f.JournalSizeDenom < 30 {
		f.JournalSizeDenom = 30
	}
	if f.JournalSizeDenom > 300 {
		f.JournalSizeDenom = 300
	}

	// can be fractional
	f.ReservedPercent = "1"
	f.BlockSize = 4096
	return nil
}

func (f *Ext4) SetVolumeProps(props StorageProperties, journalProps StorageProperties) error {
	f.SkipDiscard = props.InitialEmpty
	return nil
}

func (f *Ext4) CalcPreferredJournalSize(dataSize int64) int64 {
	// 1024 block, plus superblock
	min := int64(f.BlockSize * 1025)
	// 1 GiB
	max := int64(1024 * 1024 * 1024)

	size := int64(float64(dataSize) / f.JournalSizeDenom)
	if size < min {
		return min
	}
	if size > max {
		return max
	}
	return ((size + f.BlockSize - 1) / f.BlockSize) * f.BlockSize
}

func (f *Ext4) SetSingleDevice(devicePath string) {
	f.mainDevice = devicePath
	f.journalDevice = ""
}

func (f *Ext4) SetWithJournal(mainDevice string, journalDevice string) {
	f.mainDevice = mainDevice
	f.journalDevice = journalDevice
}

func (f *Ext4) Format(ctx context.Context, tr transport.Transport) error {
	cmd := []string{"mkfs.ext4"}

	cmd = append(cmd, "-b", fmt.Sprintf("%d", f.BlockSize))

	if f.journalDevice != "" {
		cmd = append(cmd, "-J", fmt.Sprintf("device=%s", f.journalDevice))
		journalCmd := []string{"mke2fs", "-O", "journal_dev"}
		journalCmd = append(journalCmd, "-b", fmt.Sprintf("%d", f.BlockSize))
		journalCmd = append(journalCmd, f.journalDevice)
		err := transport.CheckedRun(ctx, tr, journalCmd)
		if err != nil {
			return fmt.Errorf("journal format: %w", err)
		}
	}

	cmd = append(cmd, "-m", f.ReservedPercent)
	cmd = append(cmd, "-i", f.BytesPerInode)

	var extended []string
	if f.SkipDiscard {
		extended = append(extended, "assume_storage_prezeroed=1", "nodiscard")
	}
	if len(extended) != 0 {
		cmd = append(cmd, "-E", strings.Join(extended, ","))
	}

	// putting device path last because alpine utils don't support options after device path
	cmd = append(cmd, f.mainDevice)

	// TODO run this in background?
	err := transport.CheckedRun(ctx, tr, cmd, transport.WithTimeout(60*time.Second))
	if err != nil {
		return fmt.Errorf("run format: %w", err)
	}
	return nil
}

func (f *Ext4) GetMountOptions() []string {
	var result []string
	if f.journalDevice != "" {
		result = append(result, fmt.Sprintf("journal_path=%s", f.journalDevice))
	}
	return result
}

func (f *Ext4) Mount(ctx context.Context, tr transport.Transport, targetPath string, opts []string) error {
	cmd := []string{"mount", "-t", "ext4"}
	for _, v := range opts {
		cmd = append(cmd, "-o", v)
	}
	if f.journalDevice != "" {
		cmd = append(cmd, "-o", fmt.Sprintf("journal_path=%s", f.journalDevice))
	}

	// can't use --source and --target options because mount in busybox does not support them
	cmd = append(cmd, f.mainDevice, targetPath)

	err := transport.CheckedRun(ctx, tr, cmd, transport.WithSudo(true))
	if err != nil {
		return err
	}
	return nil
}

func (f *Ext4) Resize(ctx context.Context, tr transport.Transport) error {
	// TODO add journal resize
	cmd := []string{"resize2fs", f.mainDevice}
	err := transport.CheckedRun(ctx, tr, cmd)
	if err != nil {
		return fmt.Errorf("resize2fs: %w", err)
	}
	return nil
}

func (f *Ext4) Serialize() (string, error) {
	encoded, err := json.Marshal(f)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (f *Ext4) Inflate(serialized string) error {
	err := json.Unmarshal([]byte(serialized), f)
	if err != nil {
		return err
	}
	return nil
}

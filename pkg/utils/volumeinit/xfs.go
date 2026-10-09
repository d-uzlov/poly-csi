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

type Xfs struct {
	JournalSizeDenom float64 `map:"journalSizeDenom" json:"journal_size_denom"`

	BlockSize              int64 `map:"-" json:"blockSize"`
	SectorSize             int64 `map:"-" json:"sectorSize"`
	SkipDiscard            bool  `map:"-" json:"skipDiscard"`
	StripeUnitBytes        int64 `map:"-" json:"stripeUnitBytes"`
	StripeWidth            int64 `map:"-" json:"stripeWidth"`
	JournalStripeUnitBytes int64 `map:"-" json:"journalStripeUnitBytes"`

	mainDevice    string `map:"-" json:"-"`
	journalDevice string `map:"-" json:"-"`
}

const xfsPrefix = "xfs"

func NewXfs() FilesystemHelper {
	return &Xfs{}
}

func (f *Xfs) Capabilities() Capabilities {
	return Capabilities{
		// 1 GiB
		MinFsSize:       1024 * 1024 * 1024,
		ExternalJournal: true,
	}
}

func (f *Xfs) Configure(params map[string]string) error {
	err := serialmaps.ParseRemove(params, xfsPrefix, true, f)
	if err != nil {
		return err
	}

	f.BlockSize = 4096
	return nil
}

func (f *Xfs) SetVolumeProps(props StorageProperties, journalProps StorageProperties) error {
	f.SectorSize = 512
	if props.AtomicSize != 0 {
		if props.AtomicSize > int64(f.BlockSize) {
			f.SectorSize = f.BlockSize
		} else {
			f.SectorSize = props.AtomicSize
		}
	}

	f.SkipDiscard = props.InitialEmpty

	f.StripeUnitBytes = f.BlockSize
	f.StripeWidth = 1
	if props.RMWSize != 0 {
		f.StripeUnitBytes = props.RMWSize
		if props.OptimalWriteSize >= props.RMWSize {
			f.StripeWidth = props.OptimalWriteSize / props.RMWSize
		}
	}

	f.JournalStripeUnitBytes = journalProps.RMWSize

	return nil
}

func (f *Xfs) CalcPreferredJournalSize(dataSize int64) int64 {
	// 64 MiB
	min := int64(64 * 1024 * 1024)
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

func (f *Xfs) SetSingleDevice(devicePath string) {
	f.mainDevice = devicePath
	f.journalDevice = ""
}

func (f *Xfs) SetWithJournal(mainDevice string, journalDevice string) {
	f.mainDevice = mainDevice
	f.journalDevice = journalDevice
}

func (f *Xfs) Format(ctx context.Context, tr transport.Transport) error {
	cmd := []string{"mkfs.xfs"}

	cmd = append(cmd, "-b", fmt.Sprintf("size=%d", f.BlockSize))
	cmd = append(cmd, "-s", fmt.Sprintf("size=%d", f.SectorSize))

	if f.SkipDiscard {
		cmd = append(cmd, "-K")
	}

	data := []string{
		fmt.Sprintf("su=%d", f.StripeUnitBytes),
		fmt.Sprintf("sw=%d", f.StripeWidth),
	}

	if len(data) != 0 {
		cmd = append(cmd, "-d", strings.Join(data, ","))
	}

	journalArgs := []string{}
	if f.journalDevice != "" {
		journalArgs = append(journalArgs,
			"logdev="+f.journalDevice,
			fmt.Sprintf("su=%d", f.JournalStripeUnitBytes),
		)
	}
	if len(journalArgs) != 0 {
		cmd = append(cmd, "-l", strings.Join(journalArgs, ","))
	}

	cmd = append(cmd, f.mainDevice)

	err := transport.CheckedRun(ctx, tr, cmd, transport.WithTimeout(60*time.Second))
	if err != nil {
		return fmt.Errorf("run format: %w", err)
	}
	return nil
}

func (f *Xfs) GetMountOptions() []string {
	var result []string
	if f.journalDevice != "" {
		result = append(result, fmt.Sprintf("logdev=%s", f.journalDevice))
	}
	return result
}

func (f *Xfs) Mount(ctx context.Context, tr transport.Transport, targetPath string, opts []string) error {
	// TODO
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

func (f *Xfs) Resize(ctx context.Context, tr transport.Transport) error {
	cmd := []string{"xfs_growfs", f.mainDevice}
	err := transport.CheckedRun(ctx, tr, cmd)
	if err != nil {
		return fmt.Errorf("xfs_growfs: %w", err)
	}
	return nil
}

func (f *Xfs) Serialize() (string, error) {
	encoded, err := json.Marshal(f)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (f *Xfs) Inflate(serialized string) error {
	err := json.Unmarshal([]byte(serialized), f)
	if err != nil {
		return err
	}
	return nil
}

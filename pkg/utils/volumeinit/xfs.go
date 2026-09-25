package volumeinit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"meoe.io/poly-csi/pkg/transport"
)

type Xfs struct {
	BlockSize       int  `json:"blockSize"`
	SectorSize      int  `json:"sectorSize"`
	SkipDiscard     bool `json:"skipDiscard"`
	StripeUnitBytes int  `json:"stripeUnitBytes"`
	StripeWidth     int  `json:"stripeWidth"`
}

func NewXfs() FilesystemHelper {
	return &Xfs{}
}

func (f *Xfs) Configure(params map[string]string, props StorageProperties) error {
	f.BlockSize = 4096
	f.SectorSize = 512
	if props.AtomicSize != 0 {
		if props.AtomicSize > int64(f.BlockSize) {
			f.SectorSize = f.BlockSize
		} else {
			f.SectorSize = int(props.AtomicSize)
		}
	}

	f.SkipDiscard = props.InitialEmpty

	f.StripeUnitBytes = f.BlockSize
	f.StripeWidth = 1
	if props.RMWSize != 0 {
		f.StripeUnitBytes = int(props.RMWSize)
		if props.OptimalWriteSize >= props.RMWSize {
			f.StripeWidth = int(props.OptimalWriteSize / props.RMWSize)
		}
	}

	return nil
}

func (f *Xfs) Format(ctx context.Context, tr transport.Transport,devicePath string) error {
	cmd := []string{"mkfs.xfs"}

	cmd = append(cmd, "-b", fmt.Sprintf("size=%d", f.BlockSize))
	cmd = append(cmd, "-s", fmt.Sprintf("size=%d", f.SectorSize))

	if f.SkipDiscard {
		cmd = append(cmd, "-K")
	}

	var data []string

	cmd = append(cmd, fmt.Sprintf("su=%d,sw=%d", f.StripeUnitBytes, f.StripeWidth))
	if len(data) != 0 {
		cmd = append(cmd, "-d", strings.Join(data, ","))
	}

	cmd = append(cmd, devicePath)

	err := transport.CheckedRun(ctx, tr, cmd, transport.WithTimeout(60*time.Second))
	if err != nil {
		return fmt.Errorf("run format: %w", err)
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

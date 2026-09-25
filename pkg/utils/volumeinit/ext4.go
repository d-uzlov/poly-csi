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
	BytesPerInode string `map:"bytesPerInode" json:"bytes_per_inode"`
	SkipDiscard   bool   `map:"-" json:"skipDiscard"`
}

const ext4Prefix = "ext4."

func NewExt4() FilesystemHelper {
	return &Ext4{}
}

func (f *Ext4) Configure(params map[string]string, props StorageProperties) error {
	err := serialmaps.ParseRemove(params, ext4Prefix, true, f)
	if err != nil {
		return err
	}

	f.SkipDiscard = props.InitialEmpty
	return nil
}

func (f *Ext4) Format(ctx context.Context, tr transport.Transport, devicePath string) error {
	cmd := []string{"mkfs.ext4"}

	blockSize := 4096
	cmd = append(cmd, "-b", fmt.Sprintf("%d", blockSize))
	// can be fractional
	reservedPercent := "1"
	cmd = append(cmd, "-m", reservedPercent)

	if f.BytesPerInode == "" {
		cmd = append(cmd, "-i", "65536")
	} else {
		cmd = append(cmd, "-i", f.BytesPerInode)
	}

	var extended []string
	if f.SkipDiscard {
		extended = append(extended, "assume_storage_prezeroed=1", "nodiscard")
	}
	if len(extended) != 0 {
		cmd = append(cmd, "-E", strings.Join(extended, ","))
	}

	// putting device path last because alpine utils don't support options after device path
	cmd = append(cmd, devicePath)

	// TODO run this in background?
	err := transport.CheckedRun(ctx, tr, cmd, transport.WithTimeout(60*time.Second))
	if err != nil {
		return fmt.Errorf("run format: %w", err)
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

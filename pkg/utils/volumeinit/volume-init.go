package volumeinit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

const PreferredPrefix = "init."

type InitConfig struct {
	FsType        string `map:"fsType"        json:"fs_type"`
	BytesPerInode string `map:"bytesPerInode" json:"bytes_per_inode"`

	// guarantees reported by storage
	InitialEmpty     bool  `map:"initialEmpty"  json:"initial_empty"`
	RMWSize          int64 `map:"rmwSize"       json:"rmw_size"`
	OptimalWriteSize int64 `map:"optimalSize"   json:"optimal_size"`
	AtomicSize       int64 `map:"atomicSize"    json:"atomic_size"`
}

func (c *InitConfig) Validate() error {
	return nil
}

type Init struct {
	place string
	tr    transport.Transport
}

func NewController(tr transport.Transport) Init {
	return Init{
		tr:    tr,
	}
}

func NewNode(tr transport.Transport) Init {
	return Init{
		tr:    tr,
	}
}

func (n *Init) RunBlock(ctx context.Context, cfg InitConfig, devicePath string) (changed bool, err error) {
	fsnip := fsutils.New(n.tr)

	fsInfo, err := fsnip.GetBlkid(ctx, devicePath)
	if err != nil {
		return false, fmt.Errorf("GetBlkid: %w", err)
	}

	if fsInfo != nil {
		logs.FromCtx(ctx).MicroOp("will not format over existing fs", "device", devicePath, "fs-type", fsInfo.FsType)
		return false, nil
	}

	logs.FromCtx(ctx).MicroOp("fs info is not found, will format fs", "device", devicePath)

	switch cfg.FsType {
	case "ext4":
		cmd := []string{"mkfs.ext4"}

		blockSize := 4096
		cmd = append(cmd, "-b", fmt.Sprintf("%d", blockSize))
		// can be fractional
		reservedPercent := "1"
		cmd = append(cmd, "-m", reservedPercent)

		if cfg.BytesPerInode == "" {
			cmd = append(cmd, "-i", "65536")
		} else {
			cmd = append(cmd, "-i", cfg.BytesPerInode)
		}

		var extended []string
		if cfg.InitialEmpty {
			extended = append(extended, "assume_storage_prezeroed=1", "nodiscard")
		}
		if len(extended) != 0 {
			cmd = append(cmd, "-E", strings.Join(extended, ","))
		}

		// putting device path last because alpine utils don't support options after device path
		cmd = append(cmd, devicePath)

		// TODO run this in background?
		err = transport.CheckedRun(ctx, n.tr, cmd, transport.WithTimeout(60*time.Second))
		if err != nil {
			return false, fmt.Errorf("mkfs.ext4: %w", err)
		}
	case "xfs":
		cmd := []string{"mkfs.xfs"}

		blockSize := 4096
		cmd = append(cmd, "-b", fmt.Sprintf("size=%d", blockSize))
		if cfg.AtomicSize != 0 {
			if cfg.AtomicSize > int64(blockSize) {
				cmd = append(cmd, "-s", fmt.Sprintf("size=%d", blockSize))
			} else {
				cmd = append(cmd, "-s", fmt.Sprintf("size=%d", cfg.AtomicSize))
			}
		} else {
			cmd = append(cmd, "-s", "size=512")
		}

		if cfg.InitialEmpty {
			// don't discard before format
			cmd = append(cmd, "-K")
		}

		var data []string
		if cfg.RMWSize != 0 {
			data = append(data, fmt.Sprintf("su=%d", cfg.RMWSize))
			sw := int64(1)
			if cfg.OptimalWriteSize >= cfg.RMWSize {
				sw = cfg.OptimalWriteSize / cfg.RMWSize
			}
			data = append(data, fmt.Sprintf("sw=%d", sw))
		}
		if len(data) != 0 {
			cmd = append(cmd, "-d", strings.Join(data, ","))
		}

		cmd = append(cmd, devicePath)

		err = transport.CheckedRun(ctx, n.tr, cmd, transport.WithTimeout(60*time.Second))
		if err != nil {
			return false, fmt.Errorf("mkfs.ext4: %w", err)
		}
	default:
		return false, fmt.Errorf("unsupported fs_type: '%s'", cfg.FsType)
	}

	logs.FromCtx(ctx).MicroOp("fs format success", "device", devicePath)

	return true, nil
}

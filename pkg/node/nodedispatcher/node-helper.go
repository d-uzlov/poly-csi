package nodedispatcher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

func ensureDevicePath(ctx context.Context, tr transport.Transport, nvi *NodeVolumeInfo, devices []string, mp multipath.Multipath) (string, error) {
	if len(devices) == 0 {
		return "", fmt.Errorf("no devices in context")
	}
	if len(devices) == 1 {
		return devices[0], nil
	}
	if mp == nil {
		return "", fmt.Errorf("multipath instance is nil")
	}

	logs.FromCtx(ctx).MicroOp("multipath.Setup: request", "devices", devices)
	serializedData, err := mp.Setup(ctx, devices)
	if err != nil {
		return "", fmt.Errorf("multipath.Setup: %w", err)
	}
	if serializedData == "" {
		return "", fmt.Errorf("multipath did not set data")
	}
	logs.FromCtx(ctx).MicroOp("multipath.Setup: success")

	device, err := mp.GetDevice(ctx, serializedData)
	if err != nil {
		return "", fmt.Errorf("multipath.GetDevice: %w", err)
	}
	if device == "" {
		return "", fmt.Errorf("multipath did not set device")
	}

	err = transport.Retry(time.Millisecond*100, time.Second, func() error {
		stat, err := tr.Cmds().Stat(ctx, device)
		if err != nil {
			// runtime errors should be self-explanatory
			return err
		}
		if !stat.FileExists {
			return fmt.Errorf("file does not exist: %s", device)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to wait for device file: %w", err)
	}

	nvi.SerializedMultipathData = serializedData
	return device, nil
}

func ensureFolderUnmounted(ctx context.Context, nvi *NodeVolumeInfo, tr transport.Transport) error {
	if nvi.FolderPath == "" {
		return nil
	}

	cmd := []string{"mountpoint", nvi.FolderPath}
	runResult, err := tr.RunCommand(ctx, cmd)
	if err != nil {
		return fmt.Errorf("check nvi.FolderPath: %w", err)
	}
	if runResult.ExitCode == 0 {
		// nvi.FolderPath is a mount point, need to unmount
		fsnip := fsutils.New(tr)
		err = fsnip.Unmount(ctx, nvi.FolderPath)
		if err != nil {
			return fmt.Errorf("unmount: %w", err)
		}
	}

	err = os.Remove(nvi.FolderPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove mount path: %w", err)
	}
	nvi.FolderPath = ""
	return nil
}

func ensureMultipathDisabled(ctx context.Context, nvi *NodeVolumeInfo, mp multipath.Multipath) error {
	if nvi.SerializedMultipathData == "" {
		return nil
	}
	if mp == nil {
		return fmt.Errorf("multipath instance is nil")
	}
	logs.FromCtx(ctx).MicroOp("multipath.Destroy: request")
	err := mp.Destroy(ctx, nvi.SerializedMultipathData)
	if err != nil {
		return err
	}
	logs.FromCtx(ctx).MicroOp("multipath.Destroy: success")

	return nil
}

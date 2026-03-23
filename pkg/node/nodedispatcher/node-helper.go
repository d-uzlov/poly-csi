package nodedispatcher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
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

func ensureFolderPath(ctx context.Context, tr transport.Transport, devFolder string, devicePath string, nvi *NodeVolumeInfo, requestMount *csi.VolumeCapability_MountVolume, targetPath string) error {
	if devicePath == "" {
		return fmt.Errorf("device path is not set")
	}

	fsnip := fsutils.New(tr, devFolder)

	fsInfo, err := fsnip.GetBlkid(ctx, devicePath)
	if err != nil {
		return fmt.Errorf("GetFsInfo: %w", err)
	}

	if fsInfo == nil {
		logs.FromCtx(ctx).MicroOp("run fs format on block device", "device", devicePath)
		err = fsnip.FormatFs(ctx, devicePath, requestMount.FsType, nil)
		if err != nil {
			return fmt.Errorf("format: %w", err)
		}
		logs.FromCtx(ctx).MicroOp("format fs: success", "device", devicePath)
	}

	logs.FromCtx(ctx).MicroOp("mount device: request", "device", devicePath)
	_, err = fsnip.EnsureMounted(ctx, devicePath, targetPath, "", requestMount.MountFlags)
	if err != nil {
		return fmt.Errorf("EnsureMounted: %w", err)
	}
	nvi.FolderPath = targetPath
	logs.FromCtx(ctx).MicroOp("mount device: success", "device", devicePath, "target-path", targetPath)
	// mi.Source is a commentary, it's not linked to a block device
	// maybe we can instead compare maj:min pair
	// if mi.Source != devicePath {
	// 	return fmt.Errorf("already mounted from different source: real %s != expected %s: mounted at %s", mi.Source, devicePath, mi.MountPoint)
	// }
	// do we need to also check that mounted filesystem type matches requested?
	// if mi.FsType != requestMount.FsType {
	// 	return nil, fmt.Errorf("fs is already mounted with different fs type: %s != %s, at mountpoint %s", mi.FsType, requestMount.FsType, mi.MountPoint)
	// }

	if !nvi.InitDone {
		err = fsnip.InitFs(ctx, nvi.FolderPath)
		if err != nil {
			return fmt.Errorf("InitFs: %w", err)
		}
		nvi.InitDone = true
		logs.FromCtx(ctx).MicroOp("init fs: success")
	}

	return nil
}

func ensureFolderUnmounted(ctx context.Context, nvi *NodeVolumeInfo, tr transport.Transport, devFolder string) error {
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
		fsnip := fsutils.New(tr, devFolder)
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

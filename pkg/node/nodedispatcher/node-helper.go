package nodedispatcher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/mount-utils"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/mounts/fsutils"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/transport"
)

func ensureDevicePath(ctx context.Context, nvi *NodeVolumeInfo, devices []string, mp multipath.Multipath) (string, error) {
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
	mc, err := mp.Setup(ctx, devices)
	if err != nil {
		return "", fmt.Errorf("multipath.Setup: %w", err)
	}
	if mc.SerializedData == "" {
		// we later determine if device is using multipath based on SerializedMultipathData
		return "", fmt.Errorf("multipath did not set data")
	}
	logs.FromCtx(ctx).MicroOp("multipath.Setup: success")
	nvi.SerializedMultipathData = mc.SerializedData
	return mc.Device, nil
}

func ensureFolderPath(ctx context.Context, tr transport.Transport, devicePath string, nvi *NodeVolumeInfo, requestMount *csi.VolumeCapability_MountVolume, targetPath string) error {
	if devicePath == "" {
		return fmt.Errorf("device path is not set")
	}

	fsnip := fsutils.New(tr)

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
	}

	nvi.FolderPath = targetPath
	logs.FromCtx(ctx).MicroOp("mount device: request", "device", devicePath)
	mi, err := fsnip.EnsureMounted(devicePath, nvi.FolderPath, "", requestMount.MountFlags)
	if err != nil {
		return fmt.Errorf("EnsureMounted: %w", err)
	}
	if mi.Source != devicePath {
		return fmt.Errorf("already mounted from different source: real %s != expected %s: mounted at %s", mi.Source, devicePath, mi.MountPoint)
	}
	// do we need to also check that mounted filesystem type matches requested?
	// if mi.FsType != requestMount.FsType {
	// 	return nil, fmt.Errorf("fs is already mounted with different fs type: %s != %s, at mountpoint %s", mi.FsType, requestMount.FsType, mi.MountPoint)
	// }
	logs.FromCtx(ctx).MicroOp("mount device: success", "device", devicePath)

	return nil
}

func ensureFolderUnmounted(nvi *NodeVolumeInfo) error {
	if nvi.FolderPath == "" {
		return nil
	}
	mh, ok := mount.NewWithoutSystemd("").(mount.MounterForceUnmounter)
	if !ok {
		panic("MounterForceUnmounter is not implemented on mount instance")
	}
	err := mh.UnmountWithForce(nvi.FolderPath, time.Second*1)
	if err != nil {
		return fmt.Errorf("unmount: %w", err)
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

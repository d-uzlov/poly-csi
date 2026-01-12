package node

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/mount-utils"
	"meoe.io/poly-csi/pkg/node/mountinfo"
	"meoe.io/poly-csi/pkg/node/mounts/fsutils"
	"meoe.io/poly-csi/pkg/node/multipath"
)

func ensureDevicePath(ctx context.Context, nvi *mountinfo.NodeVolumeInfo, mp multipath.Multipath) error {
	if len(nvi.MounterContext.Devices) == 0 {
		return fmt.Errorf("no devices in context")
	}
	if len(nvi.MounterContext.Devices) == 1 {
		nvi.DevicePath = nvi.MounterContext.Devices[0]
		return nil
	}
	if mp == nil {
		return fmt.Errorf("multipath instance is nil")
	}

	mc, err := mp.Setup(ctx, nvi.MounterContext.Devices)
	if err != nil {
		return fmt.Errorf("multipath.Setup: %w", err)
	}
	nvi.MultipathContext = *mc
	nvi.DevicePath = mc.Device
	nvi.DeviceIsMultipath = true
	return nil
}

func ensureFolderPath(ctx context.Context, nvi *mountinfo.NodeVolumeInfo, requestMount *csi.VolumeCapability_MountVolume, targetPath string) error {
	if nvi.DevicePath == "" {
		return fmt.Errorf("device path is not set")
	}

	fsInfo, err := fsutils.GetBlkid(ctx, nvi.DevicePath)
	if err != nil {
		return fmt.Errorf("GetFsInfo: %w", err)
	}

	if fsInfo == nil {
		err = fsutils.FormatFs(ctx, nvi.DevicePath, requestMount.FsType, nil)
		if err != nil {
			return fmt.Errorf("format: %w", err)
		}
	}

	nvi.FolderPath = targetPath
	mi, err := fsutils.EnsureMounted(nvi.DevicePath, nvi.FolderPath, "", requestMount.MountFlags)
	if err != nil {
		return fmt.Errorf("EnsureMounted: %w", err)
	}
	if mi.Source != nvi.DevicePath {
		return fmt.Errorf("already mounted from different source: real %s != expected %s: mounted at %s", mi.Source, nvi.DevicePath, mi.MountPoint)
	}
	// do we need to also check that mounted filesystem type matches requested?
	// if mi.FsType != requestMount.FsType {
	// 	return nil, fmt.Errorf("fs is already mounted with different fs type: %s != %s, at mountpoint %s", mi.FsType, requestMount.FsType, mi.MountPoint)
	// }

	return nil
}

func ensureFolderUnmounted(nvi *mountinfo.NodeVolumeInfo) error {
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

func ensureMultipathDisabled(ctx context.Context, nvi *mountinfo.NodeVolumeInfo, mp multipath.Multipath) error {
	if !nvi.DeviceIsMultipath {
		nvi.DevicePath = ""
		return nil
	}
	if mp == nil {
		return fmt.Errorf("multipath instance is nil")
	}
	err := mp.Destroy(ctx, nvi.MultipathContext)
	if err != nil {
		return err
	}

	return nil
}

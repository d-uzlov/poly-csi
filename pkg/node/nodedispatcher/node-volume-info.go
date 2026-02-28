package nodedispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/multipath"
)

type NodeVolumeInfo struct {
	// Initialized indicates whether volume has been fully initialized.
	// Partially-initialized volumes have this as false.
	// Initialized bool `json:"initialized"` // TODO make it useful or remove

	// MounterId holds id of the mounter, to be used on disconnect and other operations.
	MounterId string `json:"mounter_id"`
	// MounterPath is a path to a folder,
	// which was created to allow mounter to store data in filesystem
	// or create mounts.
	// TODO remove this? mounters should not manually store data in filesystem
	MounterPath string `json:"mounter_path"`
	// Folder path is a path to the main volume mount
	FolderPath string `json:"folder_path"`
	// SerializedMultipathData contains serialized data returned by multipath module
	SerializedMultipathData string `json:"multipath_data"`
	// ManagedOps is copied from mounter ConnectResult
	ManagedOps mounts.ManagedOperations `json:"managed_ops"`
	// ManagedOps contains serialized data returned by mounter connect
	SerializedMounterData string `json:"mounter_data"`

	parentFolder           string   `json:"-"`
	intermediateDeviceList []string `json:"-"`
}

// a bunch of constants defining file names at staging path
const (
	metadataFile     = "/mount-info.json"
	mounterFolder    = "/mounter_staging"
	managedMountPath = "/fs_mount"
)

func newVolumeHandle(parentFolder string) NodeVolumeInfo {
	return NodeVolumeInfo{
		parentFolder: parentFolder,
	}
}

func ParseNodeVolumeInfo(parentFolder string) (*NodeVolumeInfo, error) {
	result, err := TryParseMountInfo(parentFolder)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("file is missing: %s", parentFolder+metadataFile)
	}
	return result, nil
}

func TryParseMountInfo(parentFolder string) (*NodeVolumeInfo, error) {
	content, err := os.ReadFile(parentFolder + metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var info NodeVolumeInfo
	err = json.Unmarshal([]byte(content), &info)
	if err != nil {
		return nil, fmt.Errorf("parsing mount info from %s: %w", parentFolder+metadataFile, err)
	}
	return &info, nil
}

func DeleteMountInfo(stagingPath string) error {
	return os.Remove(stagingPath + metadataFile)
}

func (v *NodeVolumeInfo) writeMetadataToDisk() error {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(v.parentFolder+metadataFile, encoded, 0660)
}

func (v *NodeVolumeInfo) mounterConnect(ctx context.Context, mounterId string, mounter mounts.NodeMounter, req mounts.ConnectRequest) error {
	v.MounterId = mounterId
	v.MounterPath = v.parentFolder + mounterFolder
	req.StagingTargetPath = v.MounterPath

	err := os.MkdirAll(v.MounterPath, 0777)
	if err != nil {
		return fmt.Errorf("create folder for mounter staging: %w", err)
	}

	cr, err := mounter.Connect(ctx, req)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	if cr == nil {
		return fmt.Errorf("connect: nil result")
	}

	v.ManagedOps = cr.ManagedOps

	if cr.MounterData != nil {
		if cr.SerializedMounterData != "" {
			return fmt.Errorf("mounter has set both MounterData and SerializedMounterData")
		}
		encoded, err := json.Marshal(cr.MounterData)
		if err != nil {
			return fmt.Errorf("serialize MounterData: %w", err)
		}
		v.SerializedMounterData = string(encoded)
	} else {
		v.SerializedMounterData = cr.SerializedMounterData
	}

	if cr.FolderPath != "" {
		v.FolderPath = cr.FolderPath
	}

	if len(cr.Devices) != 0 {
		v.intermediateDeviceList = cr.Devices
	}

	return nil
}

func (v *NodeVolumeInfo) ensureFolderPath(ctx context.Context, mp multipath.Multipath, mountRequest *csi.VolumeCapability_MountVolume) error {
	if v.FolderPath != "" {
		return nil
	}

	if !v.ManagedOps.MountFs {
		return fmt.Errorf("mounter did not set FolderPath and did not request automatic mount")
	}

	// ensureDevicePath will setup multipath if needed
	device, err := ensureDevicePath(ctx, v, v.intermediateDeviceList, mp)
	if err != nil {
		return fmt.Errorf("ensureDevicePath: %w", err)
	}

	mountPath := v.parentFolder + managedMountPath
	err = ensureFolderPath(ctx, device, v, mountRequest, mountPath)
	if err != nil {
		return fmt.Errorf("ensureFolderPath: %w", err)
	}
	// logger.Info("ManagedOps: mounted /fs_mount")

	v.FolderPath = mountPath

	return nil
}

func (v *NodeVolumeInfo) cleanup(ctx context.Context, mp multipath.Multipath, mounter mounts.NodeMounter) error {
	var err error

	if v.ManagedOps.UnmountFs {
		err = ensureFolderUnmounted(v)
		if err != nil {
			return fmt.Errorf("ensureFolderUnmounted: %w", err)
		}
		// logger.Info("ManagedOps: successfully unmounted folder")
	}

	err = ensureMultipathDisabled(ctx, v, mp)
	if err != nil {
		return fmt.Errorf("ensureMultipathDisabled: %w", err)
	}
	// logger.Info("ManagedOps: multipath disabled OK")

	err = mounter.Disconnect(ctx, v.SerializedMounterData)
	if err != nil {
		return fmt.Errorf("mounter.Disconnect: %w", err)
	}
	// logger.Info("mounter.Disconnect: success")
	if v.FolderPath != "" {
		err = os.Remove(v.FolderPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove mounted volume folder: %w", err)
		}
		v.FolderPath = ""
	}

	err = os.Remove(v.MounterPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove mounter staging dir: %w", err)
	}

	return nil
}

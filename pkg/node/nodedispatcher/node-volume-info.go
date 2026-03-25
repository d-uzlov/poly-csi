package nodedispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
	"meoe.io/poly-csi/pkg/utils/volumeinit"
)

type NodeVolumeInfo struct {
	// Initialized indicates whether volume has been fully initialized.
	// Partially-initialized volumes have this as false.
	// Initialized bool `json:"initialized"` // TODO make it useful or remove

	// ConnectorId holds id of the connector, to be used on disconnect and other operations.
	ConnectorId string `json:"connector_id"`
	// ConnectorPath is a path to a folder,
	// which was created to allow connector to store data in filesystem
	// or create mounts.
	// TODO remove this? connectors should not manually store data in filesystem
	ConnectorPath string `json:"connector_path"`

	InitDone bool `json:"initDone"`
	// Folder path is a path to the main volume mount
	FolderPath string `json:"folder_path"`
	// SerializedMultipathData contains serialized data returned by multipath module
	SerializedMultipathData string `json:"multipath_data"`
	// ManagedOps is copied from connector ConnectResult
	ManagedOps connectors.ManagedOperations `json:"managed_ops"`
	// ManagedOps contains serialized data returned by connector connect
	SerializedConnectorData string `json:"connector_data"`

	parentFolder           string   `json:"-"`
	intermediateDeviceList []string `json:"-"`
}

// a bunch of constants defining file names at staging path
const (
	metadataFile     = "/mount-info.json"
	connectorFolder  = "/connector_staging"
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

func (v *NodeVolumeInfo) connect(ctx context.Context, connectorId string, connector connectors.Connector, req connectors.ConnectRequest) error {
	v.ConnectorId = connectorId
	v.ConnectorPath = v.parentFolder + connectorFolder
	req.StagingTargetPath = v.ConnectorPath

	err := os.MkdirAll(v.ConnectorPath, 0777)
	if err != nil {
		return fmt.Errorf("create folder for connector staging: %w", err)
	}

	logs.FromCtx(ctx).MicroOp(fmt.Sprintf("connector(%s).Connect: request", connectorId))
	cr, err := connector.Connect(ctx, req)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	if cr == nil {
		return fmt.Errorf("connect: nil result")
	}
	logs.FromCtx(ctx).MicroOp(fmt.Sprintf("connector(%s).Connect: success", connectorId), "devices", cr.Devices, "folder", cr.FolderPath)

	v.ManagedOps = cr.ManagedOps

	if cr.ConnectorData != nil {
		if cr.SerializedConnectorData != "" {
			return fmt.Errorf("connector has set both ConnectorData and serializedConnectorData")
		}
		encoded, err := json.Marshal(cr.ConnectorData)
		if err != nil {
			return fmt.Errorf("serialize ConnectorData: %w", err)
		}
		v.SerializedConnectorData = string(encoded)
	} else {
		v.SerializedConnectorData = cr.SerializedConnectorData
	}

	if cr.FolderPath != "" {
		v.FolderPath = cr.FolderPath
	}

	if len(cr.Devices) != 0 {
		v.intermediateDeviceList = cr.Devices
	}

	return nil
}

func (v *NodeVolumeInfo) ensureFolderPath(ctx context.Context, tr transport.Transport, devFolder string, mp multipath.Multipath, mountRequest *csi.VolumeCapability_MountVolume, initConfig volumeinit.InitConfig) error {
	if v.FolderPath != "" {
		return nil
	}

	if !v.ManagedOps.MountFs {
		return fmt.Errorf("connector did not set FolderPath and did not request automatic mount")
	}

	// ensureDevicePath will setup multipath if needed
	devicePath, err := ensureDevicePath(ctx, tr, v, v.intermediateDeviceList, mp)
	if err != nil {
		return fmt.Errorf("ensureDevicePath: %w", err)
	}
	sr, err := tr.Cmds().Stat(ctx, devicePath)
	if err != nil {
		return fmt.Errorf("failed to check device path at %s: %w", devicePath, err)
	}
	if !sr.FileExists {
		return fmt.Errorf("checking device path at %s: file does not exist", devicePath)
	}
	if !sr.IsBlock() {
		return fmt.Errorf("checking device path at %s: file is not a block device", devicePath)
	}

	mountPath := v.parentFolder + managedMountPath

	fsnip := fsutils.New(tr)

	init := volumeinit.NewNode(map[string][]string{}, tr)
	err = init.RunBlock(ctx, initConfig, devicePath)
	if err != nil {
		return fmt.Errorf("init block device: %w", err)
	}
	logs.FromCtx(ctx).MicroOp("init block device: success")

	_, err = fsnip.EnsureMounted(ctx, devicePath, mountPath, "", mountRequest.MountFlags)
	if err != nil {
		return fmt.Errorf("EnsureMounted: %w", err)
	}
	v.FolderPath = mountPath
	logs.FromCtx(ctx).MicroOp("mount device: success", "device", devicePath, "target-path", mountPath)

	err = init.RunFs(ctx, initConfig, mountPath)
	if err != nil {
		return fmt.Errorf("init fs: %w", err)
	}
	logs.FromCtx(ctx).MicroOp("init fs: success")

	v.FolderPath = mountPath

	return nil
}

func (v *NodeVolumeInfo) cleanup(ctx context.Context, tr transport.Transport, mp multipath.Multipath, connector connectors.Connector) error {
	var err error
	logger := logs.FromCtx(ctx)

	if v.ManagedOps.UnmountFs {
		err = ensureFolderUnmounted(ctx, v, tr)
		if err != nil {
			return fmt.Errorf("ensureFolderUnmounted: %w", err)
		}
		logger.MicroOp("ManagedOps: successfully unmounted folder")
	}

	if v.SerializedMultipathData != "" {
		err = ensureMultipathDisabled(ctx, v, mp)
		if err != nil {
			return fmt.Errorf("ensureMultipathDisabled: %w", err)
		}
		logger.MicroOp("ManagedOps: multipath disabled OK")
	}

	if v.SerializedConnectorData != "" {
		err = connector.Disconnect(ctx, v.SerializedConnectorData)
		if err != nil {
			return fmt.Errorf("connector.Disconnect: %w", err)
		}
		logger.MicroOp("connector.Disconnect: success")
	} else {
		logger.MicroOp("connector.Disconnect: skipping because we don't have serialized connector data")
	}

	if v.FolderPath != "" {
		err = os.Remove(v.FolderPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove mounted volume folder: %w", err)
		}
		v.FolderPath = ""
	}

	err = os.Remove(v.ConnectorPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove connector staging dir: %w", err)
	}
	logger.MicroOp("remove connector staging dir: success")

	return nil
}

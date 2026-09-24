package nodedispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
	"meoe.io/poly-csi/pkg/utils/volumeinit"
)

const (
	mountTypeBlock  = "__mount_type_block"
	mountTypeFolder = "__mount_type_folder"
)

type NodeVolumeInfo struct {
	// ConnectorId holds id of the connector, so that deserialization can create an instance.
	ConnectorId              string            `json:"connector_id"`
	MountType                string            `json:"mount_type"`
	FsRootDirMode            string            `json:"fs_root_dir_mode"`
	FsMountPath              string            `json:"folder_path"`
	SerializedMultipathData  string            `json:"multipath_data"`
	SerializedConnectorState string            `json:"connector_data"`
	Preset                   map[string]string `json:"preset"`

	tr           transport.Transport         `json:"-"`
	connector    connectors.Connector        `json:"-"`
	conState     connectors.ConnectionHandle `json:"-"`
	mp           multipath.Multipath         `json:"-"`
	parentFolder string                      `json:"-"`
}

type SavedParams struct {
}

const metadataFile = "/mount-info.json"

func newVolumeHandle(parentFolder string, tr transport.Transport, connector connectors.Connector, mp multipath.Multipath, connectorId string, preset map[string]string) NodeVolumeInfo {
	return NodeVolumeInfo{
		parentFolder: parentFolder,
		Preset:       preset,
		tr:           tr,
		connector:    connector,
		mp:           mp,
		ConnectorId:  connectorId,
	}
}

func ParseNodeVolumeInfo(ctx context.Context, parentFolder string, tr transport.Transport, mp multipath.Multipath, devFolder string, findConnector func(connectorId string) *connectorfactory.ParsedConnector) (*NodeVolumeInfo, error) {
	result, err := TryParseMountInfo(ctx, parentFolder, tr, mp, devFolder, findConnector)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("file is missing: %s", parentFolder+metadataFile)
	}
	return result, nil
}

func TryParseMountInfo(ctx context.Context, parentFolder string, tr transport.Transport, mp multipath.Multipath, devFolder string, findConnector func(connectorId string) *connectorfactory.ParsedConnector) (*NodeVolumeInfo, error) {
	// TODO there are a lot of possibly recoverable failure conditions
	// it would be good to recover them instead of simply returning an error
	content, err := os.ReadFile(parentFolder + metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result NodeVolumeInfo
	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		return nil, fmt.Errorf("parsing mount info from %s: %w", parentFolder+metadataFile, err)
	}

	conTemplate := findConnector(result.ConnectorId)
	if conTemplate == nil {
		return nil, fmt.Errorf("createConnector 'id=%s': not found", result.ConnectorId)
	}

	connector := conTemplate.Inflate()
	if connector == nil {
		return nil, fmt.Errorf("createConnector 'id=%s': inflate returned nil", result.ConnectorId)
	}

	err = connector.Configure(ctx, connectors.CommonConfig{
		Tr:        tr,
		DevFolder: devFolder,
	}, conTemplate.Conf.ByteConfig)
	if err != nil {
		return nil, fmt.Errorf("connector.Configure: %w", err)
	}

	state, err := connector.ParseHandle(result.SerializedConnectorState)
	if err != nil {
		return nil, fmt.Errorf("connector.ParseState(%s): %w", result.SerializedConnectorState, err)
	}
	err = state.Validate()
	if err != nil {
		return nil, fmt.Errorf("connector.State.Validate: %w", err)
	}
	result.conState = state

	result.parentFolder = parentFolder
	result.tr = tr
	result.connector = connector
	result.mp = mp
	return &result, nil
}

func (v *NodeVolumeInfo) getMainBlockDevice(ctx context.Context) (string, error) {
	if v.MountType != mountTypeBlock {
		return "", fmt.Errorf("trying to get block device on non-block connection")
	}
	if v.SerializedMultipathData != "" {
		return v.mp.GetDevice(ctx, v.SerializedMultipathData)
	}
	if v.conState == nil {
		return "", fmt.Errorf("connection state object is not initialized")
	}
	devices, err := v.conState.GetDevices(ctx)
	if err != nil {
		return "", err
	}
	if len(devices) == 0 {
		return "", fmt.Errorf("state.GetDevices did not return any devices")
	}
	if len(devices) > 1 {
		return "", fmt.Errorf("state.GetDevices returned several devices, but multipath is not set up: %v", devices)
	}
	return devices[0], nil
}

func (v *NodeVolumeInfo) deleteFromDisk(ctx context.Context) error {
	err := os.Remove(v.parentFolder + metadataFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logs.FromCtx(ctx).Warn("underlying file does not exist", "path", v.parentFolder+metadataFile)
			return nil
		}
		return err
	}
	return nil
}

func (v *NodeVolumeInfo) writeToDisk() error {
	serialized, err := v.conState.Serialize()
	if err != nil {
		return fmt.Errorf("connector.State.Serialize: %w", err)
	}
	v.SerializedConnectorState = serialized

	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	// todo do this via transport
	return os.WriteFile(v.parentFolder+metadataFile, encoded, 0660)
}

func (v *NodeVolumeInfo) stageBlock(ctx context.Context) error {
	err := v.conState.ConnectBlock(ctx)
	if err != nil {
		return fmt.Errorf("conState.ConnectBlock: %w", err)
	}

	devices, err := v.conState.GetDevices(ctx)
	if err != nil {
		return fmt.Errorf("GetDevices: %w", err)
	}
	if len(devices) == 0 {
		return fmt.Errorf("connector did not set device list")
	}
	logs.FromCtx(ctx).MicroOp("got device list", "devices", devices)

	v.MountType = mountTypeBlock

	fsnip := fsutils.New(v.tr)

	devices = fsnip.WaitForBlockDeviceList(ctx, devices, time.Second, time.Millisecond*50)
	if len(devices) == 0 {
		return fmt.Errorf("connector device list has no valid devices")
	}
	logs.FromCtx(ctx).MicroOp("got device list", "devices", devices)

	if len(devices) == 1 {
		return nil
	}

	logs.FromCtx(ctx).MicroOp("multipath.Setup: request", "devices", devices)
	mpData, err := v.mp.Setup(ctx, devices)
	if err != nil {
		return fmt.Errorf("multipath.Setup: %w", err)
	}
	if mpData == "" {
		return fmt.Errorf("multipath did not set data")
	}
	logs.FromCtx(ctx).MicroOp("multipath.Setup: success")

	v.SerializedMultipathData = mpData

	device, err := v.mp.GetDevice(ctx, mpData)
	if err != nil {
		return fmt.Errorf("multipath.GetDevice: %w", err)
	}
	if device == "" {
		return fmt.Errorf("multipath did not set device")
	}

	err = fsnip.WaitForBlockDevice(ctx, device, time.Second, time.Millisecond*50)
	if err != nil {
		return fmt.Errorf("wait for device: %w", err)
	}

	return nil
}

func (v *NodeVolumeInfo) stageBlockMount(ctx context.Context, targetPath string, mountFlags []string, initConfig volumeinit.InitConfig) error {
	var err error
	v.FsMountPath = targetPath

	init := volumeinit.NewNode(v.tr)

	fsnip := fsutils.New(v.tr)

	dev, err := v.getMainBlockDevice(ctx)
	if err != nil {
		return fmt.Errorf("getMainBlockDevice: %w", err)
	}

	blockFormatStart := time.Now()
	_, err = init.RunBlock(ctx, initConfig, dev)
	if err != nil {
		return fmt.Errorf("init.RunBlock: %w", err)
	}
	logs.FromCtx(ctx).MicroOp("init.RunBlock: success", "duration", time.Since(blockFormatStart).String())

	_, err = fsnip.EnsureMounted(ctx, dev, v.FsMountPath, "", mountFlags)
	if err != nil {
		return fmt.Errorf("EnsureMounted: %w", err)
	}
	logs.FromCtx(ctx).MicroOp("mount device: success", "device", dev, "target-path", v.FsMountPath)

	cmd := []string{"chmod", v.FsRootDirMode, v.FsMountPath}
	err = transport.CheckedRun(ctx, v.tr, cmd, transport.WithSudo(true))
	if err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	return nil
}

// TODO use
func (v *NodeVolumeInfo) adjustPermissions(ctx context.Context, group string) error {
	fsAdjStart := time.Now()
	sr, err := v.tr.Cmds().Stat(ctx, v.FsMountPath)
	if err != nil {
		return fmt.Errorf("stat path: %w", err)
	}
	if !sr.FileExists {
		return fmt.Errorf("target path does not exist")
	}
	if group == "" || group == sr.Gid {
		return nil
	}
	err = v.tr.SudoCmds().Chown(ctx, v.FsMountPath, sr.Uid, group, true)
	if err != nil {
		return fmt.Errorf("chown: %w", err)
	}
	logs.FromCtx(ctx).MicroOp("chown: success", "duration", time.Since(fsAdjStart).String())
	return nil
}

func (v *NodeVolumeInfo) stageFolderMount(ctx context.Context, req connectors.ConnectFolderRequest) error {
	v.FsMountPath = req.FsMountPath
	err := os.MkdirAll(v.FsMountPath, 0777)
	if err != nil {
		return fmt.Errorf("create FsMountPath: %w", err)
	}

	err = v.conState.ConnectFolder(ctx, req)
	if err != nil {
		return fmt.Errorf("runConnect: %w", err)
	}

	v.MountType = mountTypeFolder

	fsnip := fsutils.New(v.tr)

	mi, err := fsnip.FindMount(ctx, v.FsMountPath)
	if err != nil {
		return fmt.Errorf("check mountpoint: %w", err)
	}
	if mi == nil {
		return fmt.Errorf("check mountpoint: not found")
	}

	return nil
}

func (v *NodeVolumeInfo) publishFolderMount(ctx context.Context, req connectors.ConnectFolderRequest) error {
	err := os.MkdirAll(req.FsMountPath, 0777)
	if err != nil {
		return fmt.Errorf("create FsMountPath: %w", err)
	}

	err = v.conState.ConnectFolder(ctx, req)
	if err != nil {
		return fmt.Errorf("runConnect: %w", err)
	}

	fsnip := fsutils.New(v.tr)

	mi, err := fsnip.FindMount(ctx, req.FsMountPath)
	if err != nil {
		return fmt.Errorf("check mountpoint: %w", err)
	}
	if mi == nil {
		return fmt.Errorf("check mountpoint: not found")
	}

	return nil
}

func (v *NodeVolumeInfo) cleanupMount(ctx context.Context) (err error) {
	if v.FsMountPath == "" {
		logs.FromCtx(ctx).MicroOp("not mounted, skipping unmount")
		return
	}

	logs.FromCtx(ctx).MicroOp("unmounting filesystem")

	fsnip := fsutils.New(v.tr)
	err = fsnip.Unmount(ctx, v.FsMountPath)
	if err != nil {
		return fmt.Errorf("unmount: %w", err)
	}

	err = v.tr.Cmds().RmDir(ctx, v.FsMountPath)
	if err != nil {
		return fmt.Errorf("remove mount path: %w", err)
	}
	v.FsMountPath = ""
	return nil
}

func (v *NodeVolumeInfo) cleanup(ctx context.Context) (err error) {
	defer func() {
		if err == nil {
			logs.FromCtx(ctx).MicroOp("cleanup success, deleting underlying file")
			err = v.deleteFromDisk(ctx)
			if err != nil {
				// todo this is a debug log, remove this later
				logs.FromCtx(ctx).Error("failed to delete from disk", "error", err)
			}
		} else {
			logs.FromCtx(ctx).Error("cleanup failure, saving partial result")
			deferErr := v.writeToDisk()
			if deferErr != nil {
				logs.FromCtx(ctx).Error("failed to save partial result", "error", deferErr)
			}
		}
	}()

	caps, err := v.connector.Capabilities(ctx)
	if err != nil {
		return fmt.Errorf("connector.Capabilities: %w", err)
	}

	switch v.MountType {
	case mountTypeBlock:
		err = v.cleanupMount(ctx)
		if err != nil {
			return fmt.Errorf("cleanupMount: %w", err)
		}

		if v.SerializedMultipathData != "" {
			err = ensureMultipathDisabled(ctx, v, v.mp)
			if err != nil {
				return fmt.Errorf("ensureMultipathDisabled: %w", err)
			}
			logs.FromCtx(ctx).MicroOp("ManagedOps: multipath disabled OK")
			v.SerializedMultipathData = ""
		}

	case mountTypeFolder:
		// managed unmount should run before disconnect
		if caps.ManagedUnmount {
			err = v.cleanupMount(ctx)
			if err != nil {
				return fmt.Errorf("cleanupMount: %w", err)
			}
		}

	default:
		// v.MountType is not initialized, nothing to do here
	}

	if v.SerializedConnectorState != "" {
		err = v.conState.Disconnect(ctx)
		if err != nil {
			return fmt.Errorf("connector.Disconnect: %w", err)
		}
		logs.FromCtx(ctx).MicroOp("connector.Disconnect: success")
		v.SerializedConnectorState = ""
	}

	// best-effort unmount, in case steps above failed
	err = v.cleanupMount(ctx)
	if err != nil {
		return fmt.Errorf("secondary cleanupMount: %w", err)
	}

	return nil
}

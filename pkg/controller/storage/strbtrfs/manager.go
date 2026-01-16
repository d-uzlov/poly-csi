package strbtrfs

import (
	"context"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/btrfs"
)

type manager struct {
	snip btrfs.Snippets
}

func New(commands map[string][]string, globalParams map[string]string) (storage.StorageManager, error) {
	cmds, err := parseCommands(commands)
	if err != nil {
		return nil, fmt.Errorf("parse commands: %w", err)
	}
	snip := btrfs.NewSnippets(cmds.btrfsCmd)
	return &manager{snip: snip}, nil
}

func (m *manager) GetCapabilities() storage.Capabilities {
	return storage.Capabilities{
		LocalVolume:    storage.VolFolder,
		OpaqueVolume:   "btrfs-subvolume",
		NativeSnapshot: true,
		LocalSend:      "btrfs-subvolume",
	}
}

func (m *manager) GetAvailableCapacity(ctx context.Context, staticParams map[string]string) (int64, error) {
	p, err := parseParams(staticParams, nil)
	if err != nil {
		return 0, fmt.Errorf("parse params: %w", err)
	}

	fsInfo, err := transport.FromContext(ctx).Cmds().Df(ctx, p.parentPath)
	if err != nil {
		return 0, fmt.Errorf("get filesystem info: %w", err)
	}

	return fsInfo.Available, nil
}

func (m *manager) ParseVolume(serialized string) (storage.Volume, error) {
	return parseVolume(serialized, m.snip)
}

func (m *manager) ParseNativeSnapshot(serialized string) (storage.Snapshot, error) {
	return parseSnapshot(serialized, m.snip)
}

func (m *manager) GetExistingVolume(ctx context.Context, req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	datasetPath := fmt.Sprintf("%s/%s", p.parentPath, req.Id)

	exists, err := m.snip.SubvolumeTest(ctx, datasetPath)
	if err != nil {
		return nil, fmt.Errorf("checking existing dataset: %w", err)
	}
	if !exists {
		return nil, nil
	}

	// TODO
	// if info.GetSize() != req.Size {
	// 	return nil, fmt.Errorf("size mismatch: %d != %d", info.GetSize(), req.Size)
	// }

	return newVolume(datasetPath, m.snip), nil
}

func (m *manager) CreateVolume(ctx context.Context, req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	containerPath := fmt.Sprintf("%s/%s", p.parentPath, req.Id)

	err = transport.FromContext(ctx).SudoCmds().Mkdir(ctx, containerPath, false)
	if err != nil {
		return nil, fmt.Errorf("create volume container folder: %w", err)
	}

	vol := newVolume(containerPath, m.snip)
	err = vol.createMainVolume(ctx)
	if err != nil {
		return nil, fmt.Errorf("create subvol: %w", err)
	}

	err = vol.setQuota(ctx, req.Size)
	if err != nil {
		return nil, fmt.Errorf("set quota: %w", err)
	}

	return vol, nil
}

func (m *manager) Receive(ctx context.Context, sourceFile string, req storage.CreateRequest, timeout time.Duration) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	containerPath := fmt.Sprintf("%s/%s", p.parentPath, req.Id)

	err = transport.FromContext(ctx).SudoCmds().Mkdir(ctx, containerPath, false)
	if err != nil {
		return nil, fmt.Errorf("create volume container folder: %w", err)
	}

	volumeName, err := m.snip.Receive(ctx, containerPath, sourceFile, timeout)
	if err != nil {
		return nil, fmt.Errorf("btrfs receive: %w", err)
	}

	vol := newVolume(containerPath, m.snip)

	if containerPath+"/"+volumeName != vol.getMainVolume() {
		err = transport.FromContext(ctx).SudoCmds().Move(ctx, containerPath+"/"+volumeName, vol.getMainVolume())
		if err != nil {
			return nil, fmt.Errorf("move received dataset: %w", err)
		}
	}

	err = vol.setQuota(ctx, req.Size)

	return newVolume(containerPath, m.snip), nil
}

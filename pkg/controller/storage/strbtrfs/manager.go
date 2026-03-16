package strbtrfs

import (
	"context"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/btrfs"
)

type manager struct {
	tr   transport.Transport
	snip btrfs.Snippets
}

func New(tr transport.Transport, commands map[string][]string, globalParams map[string]string) (storage.StorageManager, error) {
	cmds, err := parseCommands(commands)
	if err != nil {
		return nil, fmt.Errorf("parse commands: %w", err)
	}
	snip := btrfs.NewSnippets(tr, cmds.btrfsCmd)
	return &manager{
		tr:   tr,
		snip: snip,
	}, nil
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

	fsInfo, err := m.tr.Cmds().Df(ctx, p.parentPath)
	if err != nil {
		return 0, fmt.Errorf("get filesystem info: %w", err)
	}

	return fsInfo.Available, nil
}

func (m *manager) ParseVolume(serialized string) (storage.Volume, error) {
	return parseVolume(serialized, m.snip)
}

func (m *manager) ParseSnapshot(serialized string) (storage.Snapshot, error) {
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

	// TODO btrfs crashes when trying to get quota info in json format
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

	err = m.tr.SudoCmds().Mkdir(ctx, containerPath, false)
	if err != nil {
		return nil, fmt.Errorf("create volume container folder: %w", err)
	}

	vol := newVolume(containerPath, m.snip)
	err = vol.createMainVolume(ctx)
	if err != nil {
		return nil, fmt.Errorf("createMainVolume: %w", err)
	}

	success := false
	defer func() {
		if success {
			return
		}
		err = vol.Delete(ctx)
		if err != nil {
			logs.FromCtx(ctx).Error("failed to delete created subvol", "error", err)
		} else {
			logs.FromCtx(ctx).Info("deleted created subvol")
		}
	}()

	err = vol.setQuota(ctx, req.Size)
	if err != nil {
		return nil, fmt.Errorf("set quota: %w", err)
	}

	success = true
	return vol, nil
}

func (m *manager) Receive(ctx context.Context, sourceFile string, req storage.CreateRequest, timeout time.Duration) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	containerPath := fmt.Sprintf("%s/%s", p.parentPath, req.Id)

	err = m.tr.SudoCmds().Mkdir(ctx, containerPath, false)
	if err != nil {
		return nil, fmt.Errorf("create volume container folder: %w", err)
	}

	volumeName, err := m.snip.Receive(ctx, containerPath, sourceFile, timeout)
	if err != nil {
		return nil, fmt.Errorf("btrfs receive: %w", err)
	}

	vol := newVolume(containerPath, m.snip)

	success := false
	defer func() {
		if success {
			return
		}
		err = vol.Delete(ctx)
		if err != nil {
			logs.FromCtx(ctx).Error("failed to delete received subvol", "error", err)
		} else {
			logs.FromCtx(ctx).Info("deleted received subvol")
		}
		if containerPath+"/"+volumeName != vol.getMainVolume() {
			_, err := m.snip.SubvolumeDelete(ctx, containerPath+"/"+volumeName)
			if err != nil {
				logs.FromCtx(ctx).Error("failed to delete received subvol", "error", err)
			} else {
				logs.FromCtx(ctx).Info("deleted received subvol")
			}
		}
	}()

	if containerPath+"/"+volumeName != vol.getMainVolume() {
		err = m.tr.SudoCmds().Move(ctx, containerPath+"/"+volumeName, vol.getMainVolume())
		if err != nil {
			return nil, fmt.Errorf("move received dataset: %w", err)
		}
	}

	err = vol.setQuota(ctx, req.Size)
	if err != nil {
		return nil, fmt.Errorf("set quota: %w", err)
	}

	success = true
	return vol, nil
}

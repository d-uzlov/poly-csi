package strbtrfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/btrfs"
)

type manager struct {
	tr   transport.Transport
	snip btrfs.Snippets
}

func New(tr transport.Transport, globalParams map[string]string) (storage.StorageManager, error) {
	snip := btrfs.NewSnippets(tr)
	return &manager{
		tr:   tr,
		snip: snip,
	}, nil
}

func (m *manager) GetCapabilities() storage.Capabilities {
	return storage.Capabilities{
		LocalPathType:    storage.VolFolder,
		OpaqueVolumeType: "btrfs-subvolume",
		NativeSnapshot:   true,
		LocalSendType:    "btrfs-subvolume",
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

func (m *manager) CreateVolume(req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	containerPath := fmt.Sprintf("%s/%s", p.parentPath, req.Id)

	return newVolume(containerPath, m.snip, m.tr), nil
}

package strfolder

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
)

type manager struct {
	tr transport.Transport
}

func New(tr transport.Transport, globalParams map[string]string) storage.StorageManager {
	return &manager{
		tr: tr,
	}
}

func (m *manager) GetCapabilities() storage.Capabilities {
	return storage.Capabilities{
		LocalPathType:  storage.VolFolder,
		OpaqueType:     "",
		NativeSnapshot: false,
	}
}

func (m *manager) GetAvailableCapacity(ctx context.Context, staticParams map[string]string) (int64, error) {
	p, err := parseParams(staticParams, nil)
	if err != nil {
		return 0, fmt.Errorf("parse params: %w", err)
	}

	fsInfo, err := m.tr.Cmds().Df(ctx, p.parentFolder)
	if err != nil {
		return 0, fmt.Errorf("get filesystem info: %w", err)
	}

	return fsInfo.Available, nil
}

func (m *manager) ParseVolume(serialized string) (storage.Volume, error) {
	return parseVolume(m.tr, serialized)
}

func (m *manager) ParseSnapshot(serialized string) (storage.Snapshot, error) {
	return nil, fmt.Errorf("native snapshots are not supported for folder storage")
}

func (m *manager) CreateVolume(req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse folder params: %w", err)
	}

	folderPath := fmt.Sprintf("%s/%s", p.parentFolder, req.Id)

	return newVolume(m.tr, folderPath), nil
}

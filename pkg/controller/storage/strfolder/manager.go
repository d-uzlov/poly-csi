package strfolder

import (
	"context"
	"fmt"
	"time"

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
		LocalVolume:    storage.VolFolder,
		OpaqueVolume:   "",
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

func (m *manager) GetExistingVolume(ctx context.Context, req storage.CreateRequest) (storage.Volume, error) {
	// TODO implement GetExistingVolume
	return nil, nil
}

func (m *manager) ParseSnapshot(serialized string) (storage.Snapshot, error) {
	return nil, fmt.Errorf("native snapshots are not supported for folder storage")
}

func (m *manager) CreateVolume(ctx context.Context, req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse folder params: %w", err)
	}

	folderPath := fmt.Sprintf("%s/%s", p.parentFolder, req.Id)

	err = m.tr.SudoCmds().Mkdir(ctx, folderPath, true)
	if err != nil {
		return nil, fmt.Errorf("create volume folder: %w", err)
	}

	return newVolume(m.tr, folderPath), nil
}

func (m *manager) Receive(ctx context.Context, sourceFile string, req storage.CreateRequest, timeout time.Duration) (storage.Volume, error) {
	return nil, fmt.Errorf("unimplemented")
}

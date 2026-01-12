package strzfsdataset

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/zfsdata"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type manager struct {
	snip zfs.Snippets
}

func New(commands map[string][]string, globalParams map[string]string) (storage.StorageManager, error) {
	cmds, err := parseCommands(commands)
	if err != nil {
		return nil, fmt.Errorf("parse commands: %w", err)
	}
	snip := zfs.NewSnippets(cmds.zfsCmd)
	return &manager{snip: snip}, nil
}

func (m *manager) GetCapabilities() storage.Capabilities {
	return storage.Capabilities{
		LocalVolume:    storage.VolFolder,
		OpaqueVolume:   "zfs-dataset",
		NativeSnapshot: true,
		LocalSend:      "zfs-dataset",
	}
}

func (m *manager) GetAvailableCapacity(ctx context.Context, staticParams map[string]string) (int64, error) {
	p, err := parseParams(staticParams, nil)
	if err != nil {
		return 0, fmt.Errorf("parse params: %w", err)
	}

	datasetInfo, err := m.snip.GetDataset(ctx, p.parentDataset)
	if err != nil {
		return 0, err
	}
	if datasetInfo == nil {
		return 0, fmt.Errorf("parent dataset is not found")
	}

	return datasetInfo.Properties.Available.GetInt()
}

func (m *manager) ParseVolume(serialized string) (storage.Volume, error) {
	zd, err := zfsdata.Parse(serialized)
	if err != nil {
		return nil, fmt.Errorf("parse zfs data: %w", err)
	}

	return newVolume(zd, m.snip), nil
}

func (m *manager) ParseNativeSnapshot(serialized string) (storage.Snapshot, error) {
	var result Snapshot
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	if result.FullName == "" {
		return nil, fmt.Errorf("zfs snapshot path is empty")
	}
	result.snip = m.snip

	return &result, nil
}

func (m *manager) GetExistingVolume(ctx context.Context, req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	datasetPath := fmt.Sprintf("%s/%s", p.parentDataset, req.Id)

	datasetInfo, err := m.snip.GetDataset(ctx, datasetPath)
	if err != nil {
		return nil, fmt.Errorf("checking existing dataset: %w", err)
	}
	if datasetInfo == nil {
		return nil, nil
	}

	if datasetInfo.GetSize() != req.Size {
		return nil, fmt.Errorf("size mismatch: %d != %d", datasetInfo.GetSize(), req.Size)
	}

	return newVolume(zfsdata.ZfsData{Path: datasetPath}, m.snip), nil
}

func (m *manager) CreateVolume(ctx context.Context, req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	datasetPath := fmt.Sprintf("%s/%s", p.parentDataset, req.Id)

	_, err = m.snip.CreateDataset(ctx, datasetPath, zfs.DatasetConfig{
		Quota:          req.Size,
		RefQuota:       req.Size,
		Reservation:    0,
		RefReservation: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("create dataset: %w", err)
	}

	return newVolume(zfsdata.ZfsData{Path: datasetPath}, m.snip), nil
}

func (m *manager) Receive(ctx context.Context, sourceFile string, req storage.CreateRequest, timeout time.Duration) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	datasetPath := fmt.Sprintf("%s/%s", p.parentDataset, req.Id)
	targetSnapshot := fmt.Sprintf("%s@init", datasetPath)

	err = m.snip.Receive(ctx, targetSnapshot, sourceFile, timeout)
	if err != nil {
		return nil, fmt.Errorf("zfs receive: %w", err)
	}
	defer m.snip.DestroySnapshot(ctx, targetSnapshot)

	err = m.snip.ConfigureDataset(ctx, datasetPath, zfs.DatasetConfig{
		Quota:          req.Size,
		RefQuota:       req.Size,
		Reservation:    0,
		RefReservation: 0,
	})

	datasetInfo, err := m.snip.GetDataset(ctx, datasetPath)
	if err != nil {
		return nil, fmt.Errorf("checking existing dataset: %w", err)
	}
	if datasetInfo.GetSize() != req.Size {
		return nil, fmt.Errorf("size mismatch: %d != %d", datasetInfo.GetSize(), req.Size)
	}

	return newVolume(zfsdata.ZfsData{Path: datasetPath}, m.snip), nil
}

package strzfsdataset

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type manager struct {
	tr   transport.Transport
	snip zfs.Snippets
}

func New(tr transport.Transport, globalParams map[string]string) (storage.StorageManager, error) {
	snip := zfs.NewSnippets(tr)
	return &manager{
		tr:   tr,
		snip: snip,
	}, nil
}

func (m *manager) GetCapabilities() storage.Capabilities {
	return storage.Capabilities{
		LocalPathType:    storage.VolFolder,
		OpaqueVolumeType: "zfs-dataset",
		NativeSnapshot:   true,
		LocalSendType:    "zfs-dataset",
	}
}

func (m *manager) GetAvailableCapacity(ctx context.Context, staticParams map[string]string) (int64, error) {
	p, err := parseParams(staticParams, nil)
	if err != nil {
		return 0, fmt.Errorf("parse params: %w", err)
	}

	info, err := m.snip.GetProperties(ctx, p.parentDataset)
	if err != nil {
		return 0, err
	}
	if info == nil {
		return 0, fmt.Errorf("parent dataset is not found")
	}

	return info.Properties.Available.GetInt()
}

func (m *manager) ParseVolume(serialized string) (storage.Volume, error) {
	return parseVolume(serialized, m.snip)
}

func (m *manager) ParseSnapshot(serialized string) (storage.Snapshot, error) {
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

func (m *manager) CreateVolume(req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	datasetPath := fmt.Sprintf("%s/%s", p.parentDataset, req.Id)

	return newVolume(datasetPath, m.snip), nil
}

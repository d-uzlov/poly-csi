package strzfszvol

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/logs"
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
		LocalVolume:    storage.VolDevice,
		OpaqueVolume:   "zfs-zvol",
		NativeSnapshot: true,
		LocalSend:      "zfs-zvol",
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

func (m *manager) GetExistingVolume(ctx context.Context, req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	datasetPath := fmt.Sprintf("%s/%s", p.parentDataset, req.Id)

	info, err := m.snip.GetProperties(ctx, datasetPath)
	if err != nil {
		return nil, fmt.Errorf("checking existing dataset: %w", err)
	}
	if info == nil {
		return nil, nil
	}

	if info.GetSize() != req.Size {
		return nil, fmt.Errorf("size mismatch: %d != %d", info.GetSize(), req.Size)
	}

	return newVolume(datasetPath, m.snip), nil
}

func (m *manager) CreateVolume(ctx context.Context, req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	datasetPath := fmt.Sprintf("%s/%s", p.parentDataset, req.Id)

	_, err = m.snip.CreateZvol(ctx, datasetPath, zfs.ZvolConfig{
		Size: req.Size,
	})
	if err != nil {
		return nil, fmt.Errorf("create zvol: %w", err)
	}
	info, err := m.snip.GetProperties(ctx, datasetPath)
	if err != nil {
		return nil, fmt.Errorf("checking created dataset: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("checking created dataset: not found")
	}
	if !info.IsZvol() {
		return nil, fmt.Errorf("created zvol %s, but it's a dataset when checked", datasetPath)
	}
	if info.GetSize() != req.Size {
		return nil, fmt.Errorf("size mismatch: %d != %d", info.GetSize(), req.Size)
	}

	return newVolume(datasetPath, m.snip), nil
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
	success := false
	defer func() {
		if success {
			return
		}
		_, err := m.snip.DestroyDataset(ctx, datasetPath)
		if err != nil {
			logs.FromCtx(ctx).Error("failed to delete received dataset", "error", err.Error())
		} else {
			logs.FromCtx(ctx).Info("deleted received dataset")
		}
	}()

	info, err := m.snip.GetProperties(ctx, datasetPath)
	if err != nil {
		return nil, fmt.Errorf("checking received volume: %w", err)
	}
	if !info.IsZvol() {
		return nil, fmt.Errorf("received volume is not a zvol")
	}
	if info.GetSize() < req.Size {
		err = m.snip.ConfigureZvol(ctx, datasetPath, zfs.ZvolConfig{
			Size: req.Size,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to set zvol size: %w", err)
		}
	}

	success = true
	return newVolume(datasetPath, m.snip), nil
}

package strzfszvol

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

	_, err = m.snip.CreateZvol(ctx, datasetPath, zfs.ZvolConfig{
		Size: req.Size,
	})
	if err != nil {
		return nil, fmt.Errorf("create zvol: %w", err)
	}
	datasetInfo, err := m.snip.GetDataset(ctx, datasetPath)
	if err != nil {
		return nil, fmt.Errorf("checking created dataset: %w", err)
	}
	if datasetInfo == nil {
		return nil, fmt.Errorf("checking created dataset: not found")
	}
	if !datasetInfo.IsZvol() {
		return nil, fmt.Errorf("created zvol %s, but it's a dataset when checked", datasetPath)
	}
	if datasetInfo.GetSize() != req.Size {
		return nil, fmt.Errorf("size mismatch: %d != %d", datasetInfo.GetSize(), req.Size)
	}

	return newVolume(zfsdata.ZfsData{Path: datasetPath}, m.snip), nil
}

func (m *manager) Receive(ctx context.Context, sourceFile string, req storage.CreateRequest, timeout time.Duration) (storage.Volume, error) {
	return nil, fmt.Errorf("unimplemented")
}

package strzfszvol

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/blockvolume"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type manager struct {
	tr   transport.Transport
	snip zfs.Snippets
}

func New(tr transport.Transport, globalParams map[string]string) (blockvolume.Manager, error) {
	snip := zfs.NewSnippets(tr)
	return &manager{
		tr:   tr,
		snip: snip,
	}, nil
}

func (m *manager) GetCapabilities() blockvolume.Capabilities {
	return blockvolume.Capabilities{
		OpaqueType:        "zfs-zvol",
		NativeSnapshot:    true,
		LocalSendType:     "zfs-zvol",
		BlockInitialEmpty: true,
	}
}

func (m *manager) GetAvailableCapacity(ctx context.Context, parser *serialmaps.Parser) (int64, error) {
	p, err := parseParams(parser)
	if err != nil {
		return 0, fmt.Errorf("parse params: %w", err)
	}

	info, err := m.snip.GetProperties(ctx, p.ParentDataset)
	if err != nil {
		return 0, err
	}
	if info == nil {
		return 0, fmt.Errorf("parent dataset is not found")
	}

	return info.Properties.Available.GetInt()
}

func (m *manager) ParseVolume(serialized json.RawMessage) (blockvolume.Volume, error) {
	return parseVolume(serialized, m.snip)
}

func (m *manager) ParseSnapshot(serialized json.RawMessage) (blockvolume.Snapshot, error) {
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

func (m *manager) CreateVolume(req blockvolume.CreateRequest) (blockvolume.Volume, error) {
	p, err := parseParams(req.Parser)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if p == nil {
		return nil, fmt.Errorf("parse params: nil result")
	}

	datasetPath := fmt.Sprintf("%s/%s", p.ParentDataset, req.Id)
	return newVolume(*p, datasetPath, m.snip), nil
}

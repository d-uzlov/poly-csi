package strzfsdataset

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/filevol"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type manager struct {
	tr   transport.Transport
	snip zfs.Snippets
}

func New(tr transport.Transport, globalParams map[string]string) (filevol.Manager, error) {
	snip := zfs.NewSnippets(tr)
	return &manager{
		tr:   tr,
		snip: snip,
	}, nil
}

func (m *manager) GetCapabilities() filevol.Capabilities {
	return filevol.Capabilities{
		OpaqueType:     "zfs-dataset",
		NativeSnapshot: true,
		LocalSendType:  "zfs-dataset",
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

func (m *manager) ParseVolume(serialized json.RawMessage) (filevol.Volume, error) {
	return parseVolume(serialized, m.snip)
}

func (m *manager) ParseSnapshot(serialized json.RawMessage) (filevol.Snapshot, error) {
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

func (m *manager) CreateVolume(req filevol.CreateRequest) (filevol.Volume, error) {
	p, err := parseParams(req.Parser)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	datasetPath := fmt.Sprintf("%s/%s", p.ParentDataset, req.Id)

	return newVolume(datasetPath, m.snip), nil
}

package strfolder

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/filevol"
	"meoe.io/poly-csi/pkg/transport"
)

type manager struct {
	tr transport.Transport
}

func New(tr transport.Transport, globalParams map[string]string) (filevol.Manager, error) {
	return &manager{
		tr: tr,
	}, nil
}

func (m *manager) GetCapabilities() filevol.Capabilities {
	return filevol.Capabilities{
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

func (m *manager) ParseVolume(serialized json.RawMessage) (filevol.Volume, error) {
	return parseVolume(m.tr, serialized)
}

func (m *manager) ParseSnapshot(serialized json.RawMessage) (filevol.Snapshot, error) {
	return nil, fmt.Errorf("native snapshots are not supported for folder storage")
}

func (m *manager) CreateVolume(req filevol.CreateRequest) (filevol.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse folder params: %w", err)
	}

	folderPath := fmt.Sprintf("%s/%s", p.parentFolder, req.Id)

	return newVolume(m.tr, folderPath), nil
}

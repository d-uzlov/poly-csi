package strbtrfs

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/filevol"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/btrfs"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type manager struct {
	tr   transport.Transport
	snip btrfs.Snippets
}

func New(tr transport.Transport, globalParams map[string]string) (filevol.Manager, error) {
	snip := btrfs.NewSnippets(tr)
	return &manager{
		tr:   tr,
		snip: snip,
	}, nil
}

func (m *manager) GetCapabilities() filevol.Capabilities {
	return filevol.Capabilities{
		OpaqueType:     "btrfs-subvolume",
		NativeSnapshot: true,
		LocalSendType:  "btrfs-subvolume",
	}
}

func (m *manager) GetAvailableCapacity(ctx context.Context, parser *serialmaps.Parser) (int64, error) {
	p, err := parseParams(parser)
	if err != nil {
		return 0, fmt.Errorf("parse params: %w", err)
	}

	fsInfo, err := m.tr.Cmds().Df(ctx, p.ParentPath)
	if err != nil {
		return 0, fmt.Errorf("get filesystem info: %w", err)
	}

	return fsInfo.Available, nil
}

func (m *manager) ParseVolume(serialized json.RawMessage) (filevol.Volume, error) {
	return parseVolume(serialized, m.snip)
}

func (m *manager) ParseSnapshot(serialized json.RawMessage) (filevol.Snapshot, error) {
	return parseSnapshot(serialized, m.snip)
}

func (m *manager) CreateVolume(req filevol.CreateRequest) (filevol.Volume, error) {
	p, err := parseParams(req.Parser)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	containerPath := fmt.Sprintf("%s/%s", p.ParentPath, req.Id)

	return newVolume(containerPath, m.snip, m.tr), nil
}

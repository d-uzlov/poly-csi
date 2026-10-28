package strlvm

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/blockvolume"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/lvm"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type manager struct {
	tr   transport.Transport
	snip lvm.Snippets
}

func New(tr transport.Transport, globalParams map[string]string) (blockvolume.Manager, error) {
	snip := lvm.NewSnippets(tr)
	return &manager{
		tr:   tr,
		snip: snip,
	}, nil
}

func (m *manager) GetCapabilities() blockvolume.Capabilities {
	return blockvolume.Capabilities{
		OpaqueType:     "lvm",
		NativeSnapshot: true,
		LocalSendType:  "",
	}
}

func (m *manager) GetAvailableCapacity(ctx context.Context, parser *serialmaps.Parser) (int64, error) {
	p, err := parseParams(parser)
	if err != nil {
		return 0, fmt.Errorf("parse params: %w", err)
	}

	fsInfo, err := m.tr.Cmds().Df(ctx, p.VgName)
	if err != nil {
		return 0, fmt.Errorf("get filesystem info: %w", err)
	}

	return fsInfo.Available, nil
}

func (m *manager) ParseVolume(serialized json.RawMessage) (blockvolume.Volume, error) {
	return parseVolume(serialized, m.snip)
}

func (m *manager) ParseSnapshot(serialized json.RawMessage) (blockvolume.Snapshot, error) {
	return parseSnapshot(serialized, m.snip)
}

func (m *manager) CreateVolume(req blockvolume.CreateRequest) (blockvolume.Volume, error) {
	p, err := parseParams(req.Parser)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	lvName := req.Id
	return newVolume(p.VgName, lvName, m.snip), nil
}

package strlvm

import (
	"context"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/lvm"
)

type manager struct {
	tr   transport.Transport
	snip lvm.Snippets
}

func New(tr transport.Transport, commands map[string][]string, globalParams map[string]string) (storage.StorageManager, error) {
	cmds, err := parseCommands(commands)
	if err != nil {
		return nil, fmt.Errorf("parse commands: %w", err)
	}
	snip := lvm.NewSnippets(tr, cmds.lvmCmd)
	return &manager{
		tr:   tr,
		snip: snip,
	}, nil
}

func (m *manager) GetCapabilities() storage.Capabilities {
	return storage.Capabilities{
		LocalVolume:    storage.VolDevice,
		OpaqueVolume:   "lvm",
		NativeSnapshot: true,
		LocalSend:      "",
	}
}

func (m *manager) GetAvailableCapacity(ctx context.Context, staticParams map[string]string) (int64, error) {
	p, err := parseParams(staticParams, nil)
	if err != nil {
		return 0, fmt.Errorf("parse params: %w", err)
	}

	fsInfo, err := m.tr.Cmds().Df(ctx, p.vgName)
	if err != nil {
		return 0, fmt.Errorf("get filesystem info: %w", err)
	}

	return fsInfo.Available, nil
}

func (m *manager) ParseVolume(serialized string) (storage.Volume, error) {
	return parseVolume(serialized, m.snip)
}

func (m *manager) ParseSnapshot(serialized string) (storage.Snapshot, error) {
	return parseSnapshot(serialized, m.snip)
}

func (m *manager) GetExistingVolume(ctx context.Context, req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	lvs, err := m.snip.ListLvs(ctx, p.vgName, req.Id)
	if err != nil {
		return nil, fmt.Errorf("checking existing lv: %w", err)
	}
	if len(lvs) == 0 {
		return nil, nil
	}
	lv := lvs[0]
	if lv.LvName != req.Id {
		return nil, nil
	}

	currentSize := lv.GetSize()
	if currentSize != req.Size {
		return nil, fmt.Errorf("size mismatch: %d != %d", currentSize, req.Size)
	}

	return newVolume(lv.VgName, lv.LvName, m.snip), nil
}

func (m *manager) CreateVolume(ctx context.Context, req storage.CreateRequest) (storage.Volume, error) {
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	lvName := req.Id

	_, err = m.snip.LvCreate(ctx, p.vgName, lvName, req.Size)
	if err != nil {
		return nil, fmt.Errorf("create lv: %w", err)
	}

	vol := newVolume(p.vgName, lvName, m.snip)

	return vol, nil
}

func (m *manager) Receive(ctx context.Context, sourceFile string, req storage.CreateRequest, timeout time.Duration) (storage.Volume, error) {
	return nil, fmt.Errorf("unimplemented")
}

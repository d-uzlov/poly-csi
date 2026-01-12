package mntrnvme

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/node/mounts"
)

type nvmeMounter struct {
	id string
}

func NewNvmeMounter(id string) mounts.NodeMounter {
	return &nvmeMounter{
		id: id,
	}
}

func (m *nvmeMounter) Connect(ctx context.Context, req mounts.ConnectRequest) (*mounts.MounterContext, error) {
	pc, err := ParsePublishContext(req.PublishContext)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}

	successCount := 0
	for _, v := range req.AddressOptions {
		err = nvmeConnect(ctx, pc.Nqn, pc.Port, pc.Transport, v.Address, pc.IoQueues)
		if err != nil {
			if len(req.AddressOptions) <= 1 {
				return nil, fmt.Errorf("nvmeConnect: %w", err)
			}

			slogctx.FromCtx(ctx).Error("nvmeConnect failed", "address", v, "error", err)
			continue
		}
		successCount++
	}
	if successCount == 0 {
		return nil, fmt.Errorf("all %d nvme multipath connections failed, see logs for details", len(req.AddressOptions))
	}

	namespaceDevice, err := nvmeFindNamespace(ctx, pc.Nqn, 1)
	if err != nil {
		return nil, fmt.Errorf("nvmeFindNamespace: %w", err)
	}

	namespaceDevicePath := "/dev/" + namespaceDevice
	deviceFound := false
	for range 10 {
		_, err = os.Stat(namespaceDevicePath)
		if err == nil {
			deviceFound = true
			break
		}
		slogctx.FromCtx(ctx).Warn("waiting for nvme device to show up", "device", namespaceDevicePath)
		if errors.Is(err, fs.ErrNotExist) {
			time.Sleep(time.Millisecond * 100)
		} else {
			return nil, fmt.Errorf("check device %s: %w", namespaceDevicePath, err)
		}
	}
	if !deviceFound {
		return nil, fmt.Errorf("nvme device not found: %s", namespaceDevicePath)
	}

	return &mounts.MounterContext{
		FolderPath: "",
		Devices:    []string{namespaceDevicePath},
		ManagedOps: mounts.ManagedOperations{
			MountFs:   true,
			UnmountFs: true,
			ExpandFs:  true,
		},
		Extra: map[string]string{
			"nvme.nqn":              pc.Nqn,
			"nvme.namespace-device": namespaceDevicePath,
		},
	}, nil
}

func (m *nvmeMounter) Disconnect(ctx context.Context, nvi mounts.MounterContext) error {
	nqn, ok := nvi.Extra["nvme.nqn"]
	if !ok {
		return fmt.Errorf("local volume info: 'nvme.nqn' missing")
	}
	err := nvmeDisconnect(ctx, nqn)
	if err != nil {
		return fmt.Errorf("nvmeDisconnect: %w", err)
	}
	return nil
}

func (m *nvmeMounter) Expand(ctx context.Context, nvi mounts.MounterContext) error {
	nqn, ok := nvi.Extra["nvme.nqn"]
	if !ok {
		return fmt.Errorf("local volume info: 'nvme.nqn' missing")
	}
	err := nvmeRescan(ctx, nqn)
	if err != nil {
		return fmt.Errorf("nvmeRescan: %w", err)
	}
	return nil
}

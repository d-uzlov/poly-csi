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
	id   string
	snip NvmeSnippets
}

func NewNvmeMounter(id string) mounts.NodeMounter {
	return &nvmeMounter{
		id:   id,
		snip: newNvmeSnippets(),
	}
}

func (m *nvmeMounter) Connect(ctx context.Context, req mounts.ConnectRequest) (*mounts.MounterContext, error) {
	pc, err := ParsePublishContext(req.PublishContext)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}

	hostKey := ""
	controllerKey := ""
	switch pc.AuthType {
	case AuthTypeNqn:
		// nothing to do
	case AuthTypeDhChap:
		secret, err := ParseSecretContext(req.PublishContextSecret)
		if err != nil {
			return nil, fmt.Errorf("ParseSecretContext: %w", err)
		}
		hostKey = secret.HostKey
		controllerKey = secret.ControllerKey
	default:
		return nil, fmt.Errorf("unknown auth type: %s", pc.AuthType)
	}

	successCount := 0
	for _, v := range req.AddressOptions {
		err = m.snip.Connect(ctx, pc.TargetNqn, pc.HostNqn, pc.HostId, pc.ConnectPort, pc.TransportType, v.Address, pc.IoQueues, hostKey, controllerKey)
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

	namespaceDevice, err := m.snip.FindNamespaceDevice(ctx, pc.TargetNqn, 1)
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
			"nvme.nqn":              pc.TargetNqn,
			"nvme.namespace-device": namespaceDevicePath,
		},
	}, nil
}

func (m *nvmeMounter) Disconnect(ctx context.Context, nvi mounts.MounterContext) error {
	nqn, ok := nvi.Extra["nvme.nqn"]
	if !ok {
		return fmt.Errorf("local volume info: 'nvme.nqn' missing")
	}
	err := m.snip.Disconnect(ctx, nqn)
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
	err := m.snip.Rescan(ctx, nqn)
	if err != nil {
		return fmt.Errorf("nvmeRescan: %w", err)
	}
	return nil
}

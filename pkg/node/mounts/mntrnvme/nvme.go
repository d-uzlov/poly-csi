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
	snip   NvmeSnippets
	config Config
}

func NewNvmeMounter(config Config) mounts.NodeMounter {
	return &nvmeMounter{
		snip:   newNvmeSnippets(config.NvmeCmd, config.DevFolder, config.ParseKernelLog),
		config: config,
	}
}

func (m *nvmeMounter) Connect(ctx context.Context, req mounts.ConnectRequest) (*mounts.ConnectResult, error) {
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

	namespaceDevicePath := m.config.DevFolder + "/" + namespaceDevice
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

	return &mounts.ConnectResult{
		FolderPath: "",
		Devices:    []string{namespaceDevicePath},
		ManagedOps: mounts.ManagedOperations{
			MountFs:   true,
			UnmountFs: true,
			ExpandFs:  true,
		},
		MounterData: VolumeData{
			Nqn: pc.TargetNqn,
		},
	}, nil
}

func (m *nvmeMounter) Disconnect(ctx context.Context, serializedMounterData string) error {
	vd, err := parseVolumeData(serializedMounterData)
	if err != nil {
		return fmt.Errorf("parse serialized mounter data: %w", err)
	}

	err = m.snip.Disconnect(ctx, vd.Nqn)
	if err != nil {
		return fmt.Errorf("nvmeDisconnect: %w", err)
	}
	return nil
}

func (m *nvmeMounter) Expand(ctx context.Context, serializedMounterData string) error {
	vd, err := parseVolumeData(serializedMounterData)
	if err != nil {
		return fmt.Errorf("parse serialized mounter data: %w", err)
	}

	err = m.snip.Rescan(ctx, vd.Nqn)
	if err != nil {
		return fmt.Errorf("nvmeRescan: %w", err)
	}
	return nil
}

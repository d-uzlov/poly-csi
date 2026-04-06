package cntrnvme

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type connector struct {
	tr        transport.Transport
	snip      NvmeSnippets
	config    Config
	devFolder string
}

func New(tr transport.Transport, config Config, devFolder string) connectors.Connector {
	return &connector{
		tr:        tr,
		snip:      newNvmeSnippets(tr, config.NvmeCmd, devFolder, config.ParseKernelLog),
		config:    config,
		devFolder: devFolder,
	}
}

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		ConnectType: connectors.ConnectTypeBlock,
	}, nil
}

func (m *connector) Connect(ctx context.Context, req connectors.ConnectRequest) (*connectors.ConnectResult, error) {
	pc, sc, err := ParsePublishContext(req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	err = validateContext(pc, sc)
	if err != nil {
		return nil, fmt.Errorf("validateContext: %w", err)
	}

	successCount := 0
	for _, v := range req.AddressOptions {
		err = m.snip.Connect(ctx, pc.TargetNqn, pc.HostNqn, pc.HostId, pc.ConnectPort, pc.TransportType, v.Address, pc.IoQueues, sc.HostKey, sc.ControllerKey)
		if err != nil {
			if len(req.AddressOptions) <= 1 {
				return nil, fmt.Errorf("nvmeConnect: %w", err)
			}

			logs.FromCtx(ctx).Error("nvmeConnect failed", "address", v, "error", err.Error())
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

	namespaceDevicePath := m.devFolder + "/" + namespaceDevice

	return &connectors.ConnectResult{
		Devices:    []string{namespaceDevicePath},
		ConnectorData: VolumeData{
			Nqn: pc.TargetNqn,
		},
	}, nil
}

func (m *connector) Disconnect(ctx context.Context, serializedConnectorData string) error {
	vd, err := parseVolumeData(serializedConnectorData)
	if err != nil {
		return fmt.Errorf("parse serialized connector data: %w", err)
	}

	err = m.snip.Disconnect(ctx, vd.Nqn)
	if err != nil {
		return fmt.Errorf("nvmeDisconnect: %w", err)
	}
	return nil
}

func (m *connector) Expand(ctx context.Context, serializedConnectorData string) error {
	vd, err := parseVolumeData(serializedConnectorData)
	if err != nil {
		return fmt.Errorf("parse serialized connector data: %w", err)
	}

	err = m.snip.Rescan(ctx, vd.Nqn)
	if err != nil {
		return fmt.Errorf("nvmeRescan: %w", err)
	}
	return nil
}

package cntrnfs

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

type connector struct {
	tr        transport.Transport
	devFolder string
}

func New(tr transport.Transport, devFolder string) connectors.Connector {
	return &connector{
		tr:        tr,
		devFolder: devFolder,
	}
}

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		ConnectType:    connectors.ConnectTypeFolder,
		ManagedUnmount: true,
	}, nil
}

func (m *connector) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	return &connectors.ConnectorNodeInfo{}, nil
}

func (m *connector) Connect(ctx context.Context, req connectors.ConnectRequest) (*connectors.ConnectResult, error) {
	if req.AccessMount == nil {
		return nil, fmt.Errorf("nfs does not support block mounting")
	}
	if len(req.AddressOptions) != 1 {
		return nil, fmt.Errorf("nfs connector does not support more than one address")
	}

	pc, sc, err := ParsePublishContext(req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	err = validateContext(pc, sc)
	if err != nil {
		return nil, fmt.Errorf("validateContext: %w", err)
	}

	nfsPath := fmt.Sprintf("%s:%s", req.AddressOptions[0].Address, pc.Path)

	mi, err := fsutils.New(m.tr).EnsureMounted(ctx, nfsPath, req.FsMountPath, req.AccessMount.FsType, req.AccessMount.MountFlags)
	if err != nil {
		return nil, err
	}
	if mi == nil {
		return nil, fmt.Errorf("mount succeeded but fs info is nil")
	}
	if !strings.HasPrefix(mi.FsType, "nfs") {
		return nil, fmt.Errorf("fs is already mounted with different fs type: %s != nfs, at mountpoint %s", mi.FsType, mi.MountPoint)
	}
	if mi.Source != nfsPath {
		return nil, fmt.Errorf("already mounted from different source: %s != %s: mounted at %s", mi.Source, nfsPath, mi.MountPoint)
	}

	return &connectors.ConnectResult{}, nil
}

func (m *connector) Disconnect(ctx context.Context, serializedConnectorData string) error {
	// unmount is managed externally
	return nil
}

func (m *connector) Expand(ctx context.Context, serializedConnectorData string) error {
	// nfs volumes don't need expanding
	return nil
}

package cntrnfs

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

type nfsConnector struct {
	tr        transport.Transport
	devFolder string
}

func New(tr transport.Transport, devFolder string) connectors.Connector {
	return &nfsConnector{
		tr:        tr,
		devFolder: devFolder,
	}
}

const nfsMountSuffix = "/nfs_mount"

func (m *nfsConnector) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	return &connectors.ConnectorNodeInfo{}, nil
}

func (m *nfsConnector) Connect(ctx context.Context, req connectors.ConnectRequest) (*connectors.ConnectResult, error) {
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

	mountTarget := req.StagingTargetPath + nfsMountSuffix
	mi, err := fsutils.New(m.tr, m.devFolder).EnsureMounted(ctx, nfsPath, mountTarget, req.AccessMount.FsType, req.AccessMount.MountFlags)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(mi.FsType, "nfs") {
		return nil, fmt.Errorf("fs is already mounted with different fs type: %s != nfs, at mountpoint %s", mi.FsType, mi.MountPoint)
	}
	if mi.Source != nfsPath {
		return nil, fmt.Errorf("already mounted from different source: %s != %s: mounted at %s", mi.Source, nfsPath, mi.MountPoint)
	}

	return &connectors.ConnectResult{
		FolderPath: mountTarget,
		ManagedOps: connectors.ManagedOperations{
			MountFs:   false,
			UnmountFs: true,
			ExpandFs:  false,
		},
	}, nil
}

func (m *nfsConnector) Disconnect(ctx context.Context, serializedConnectorData string) error {
	// unmount is managed externally
	return nil
}

func (m *nfsConnector) Expand(ctx context.Context, serializedConnectorData string) error {
	// nfs volumes don't need expanding
	return nil
}

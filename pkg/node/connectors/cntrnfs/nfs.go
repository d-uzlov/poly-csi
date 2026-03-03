package cntrnfs

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/mounts/fsutils"
	"meoe.io/poly-csi/pkg/transport"
)

type nfsConnector struct {
	tr transport.Transport
}

func New(tr transport.Transport) connectors.Connector {
	return &nfsConnector{
		tr: tr,
	}
}

const nfsMountSuffix = "/nfs_mount"

func (m *nfsConnector) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	return &connectors.ConnectorNodeInfo{}, nil
}

func (m *nfsConnector) Connect(ctx context.Context, req connectors.ConnectRequest) (*connectors.ConnectResult, error) {
	pc, err := ParsePublishContext(req.PublishContext)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	if req.AccessMount == nil {
		return nil, fmt.Errorf("nfs does not support block mounting")
	}
	if len(req.AddressOptions) != 1 {
		return nil, fmt.Errorf("nfs connector does not support more than one address")
	}
	nfsPath := fmt.Sprintf("%s:%s", req.AddressOptions[0].Address, pc.Path)

	mountTarget := req.StagingTargetPath + nfsMountSuffix
	mi, err := fsutils.New(m.tr).EnsureMounted(nfsPath, mountTarget, req.AccessMount.FsType, req.AccessMount.MountFlags)
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

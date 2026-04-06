package cntrcifs

import (
	"context"
	"fmt"

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
		ConnectType:    connectors.ConnectTypePublishFolder,
		ManagedUnmount: true,
	}, nil
}

func (c *connector) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	return &connectors.ConnectorNodeInfo{}, nil
}

func (c *connector) Connect(ctx context.Context, req connectors.ConnectRequest) (*connectors.ConnectResult, error) {
	if req.AccessMount == nil {
		return nil, fmt.Errorf("smb/cifs does not support block mounting")
	}
	if len(req.AddressOptions) != 1 {
		return nil, fmt.Errorf("smb connector does not support more than one address")
	}
	pc, sc, err := ParsePublishContext(req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	err = validateContext(pc, sc)
	if err != nil {
		return nil, fmt.Errorf("validateContext: %w", err)
	}

	smbPath := fmt.Sprintf("//%s/%s", req.AddressOptions[0].Address, pc.ShareName)

	flags := req.AccessMount.MountFlags
	if req.AccessMount.VolumeMountGroup != "" {
		flags = append(req.AccessMount.MountFlags, fmt.Sprintf("gid=%s", req.AccessMount.VolumeMountGroup))
	}
	flags = append(flags, fmt.Sprintf("username=%s", pc.UserName), fmt.Sprintf("password=%s", sc.UserPassword))

	if req.AccessMount.FsType != "cifs" {
		return nil, fmt.Errorf("expected cifs fs type but got %s", req.AccessMount.FsType)
	}
	mi, err := fsutils.New(c.tr).EnsureMounted(ctx, smbPath, req.FsMountPath, req.AccessMount.FsType, flags)
	if err != nil {
		return nil, err
	}
	if mi == nil {
		return nil, fmt.Errorf("mount succeeded but fs info is nil")
	}
	if mi.FsType != "cifs" {
		return nil, fmt.Errorf("fs is already mounted with different fs type: %s != cifs, at mountpoint %s", mi.FsType, mi.MountPoint)
	}
	if mi.Source != smbPath {
		return nil, fmt.Errorf("already mounted from different source: %s != %s: mounted at %s", mi.Source, smbPath, mi.MountPoint)
	}

	return &connectors.ConnectResult{}, nil
}

func (c *connector) Disconnect(ctx context.Context, serializedConnectorData string) error {
	// unmount is managed externally
	return nil
}

func (c *connector) Expand(ctx context.Context, serializedConnectorData string) error {
	// cifs volumes don't need expanding
	return nil
}

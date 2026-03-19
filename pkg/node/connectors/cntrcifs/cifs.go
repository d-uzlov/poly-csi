package cntrcifs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/mounts/fsutils"
	"meoe.io/poly-csi/pkg/transport"
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

const mountSuffix = "/smb_mount"

func (c *connector) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	return &connectors.ConnectorNodeInfo{}, nil
}

func (c *connector) Connect(ctx context.Context, req connectors.ConnectRequest) (*connectors.ConnectResult, error) {
	pc, err := ParsePublishContext(req.PublishContext, req.PublishContextSecret)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	if req.AccessMount == nil {
		return nil, fmt.Errorf("smb/cifs does not support block mounting")
	}
	if len(req.AddressOptions) != 1 {
		return nil, fmt.Errorf("smb connector does not support more than one address")
	}
	credFile := req.StagingTargetPath + "/creds"
	credContent := fmt.Sprintf("username=%s\npassword=%s", pc.UserName, pc.UserPassword)
	err = c.tr.Cmds().Write(ctx, credFile, []byte(credContent))
	if err != nil {
		return nil, fmt.Errorf("create credential file: %w", err)
	}

	smbPath := fmt.Sprintf("//%s/%s", req.AddressOptions[0].Address, pc.ShareName)

	flags := req.AccessMount.MountFlags

	if req.AccessMount.VolumeMountGroup != "" {
		flags = append(req.AccessMount.MountFlags, fmt.Sprintf("gid=%s", req.AccessMount.VolumeMountGroup))
	}

	if req.AccessMount.FsType != "cifs" {
		return nil, fmt.Errorf("expected cifs fs type but got %s", req.AccessMount.FsType)
	}
	mountTarget := req.StagingTargetPath + mountSuffix
	flags = append(req.AccessMount.MountFlags, fmt.Sprintf("credentials=%s", credFile))
	mi, err := fsutils.New(c.tr, c.devFolder).EnsureMounted(smbPath, mountTarget, req.AccessMount.FsType, flags)
	if err != nil {
		return nil, err
	}
	if mi.FsType != "cifs" {
		return nil, fmt.Errorf("fs is already mounted with different fs type: %s != cifs, at mountpoint %s", mi.FsType, mi.MountPoint)
	}
	if mi.Source != smbPath {
		return nil, fmt.Errorf("already mounted from different source: %s != %s: mounted at %s", mi.Source, smbPath, mi.MountPoint)
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

func (c *connector) Disconnect(ctx context.Context, serializedConnectorData string) error {
	// unmount is managed externally
	return nil
}

func (c *connector) Expand(ctx context.Context, serializedConnectorData string) error {
	// cifs volumes don't need expanding
	return nil
}

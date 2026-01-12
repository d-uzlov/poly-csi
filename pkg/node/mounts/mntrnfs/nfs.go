package mntrnfs

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/fsutils"
)

type nfsMounter struct {
	id string
}

func NewNfsMounter(id string) mounts.NodeMounter {
	return &nfsMounter{
		id: id,
	}
}

const nfsMountSuffix = "/nfs_mount"

func (m *nfsMounter) GetInfo(ctx context.Context) (*mounts.MounterNodeInfo, error) {
	return &mounts.MounterNodeInfo{
		Id: m.id,
	}, nil
}

func (m *nfsMounter) Connect(ctx context.Context, req mounts.ConnectRequest) (*mounts.MounterContext, error) {
	pc, err := ParsePublishContext(req.PublishContext)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	if req.AccessMount == nil {
		return nil, fmt.Errorf("nfs does not support block mounting")
	}
	if len(req.AddressOptions) != 1 {
		return nil, fmt.Errorf("nfs mounter does not support more than one address")
	}
	nfsPath := fmt.Sprintf("%s:%s", req.AddressOptions[0].Address, pc.Path)

	mountTarget := req.StagingTargetPath + nfsMountSuffix
	mi, err := fsutils.EnsureMounted(nfsPath, mountTarget, req.AccessMount.FsType, req.AccessMount.MountFlags)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(mi.FsType, "nfs") {
		return nil, fmt.Errorf("fs is already mounted with different fs type: %s != nfs, at mountpoint %s", mi.FsType, mi.MountPoint)
	}
	if mi.Source != nfsPath {
		return nil, fmt.Errorf("already mounted from different source: %s != %s: mounted at %s", mi.Source, nfsPath, mi.MountPoint)
	}

	return &mounts.MounterContext{
		FolderPath: mountTarget,
		ManagedOps: mounts.ManagedOperations{
			MountFs:   false,
			UnmountFs: true,
			ExpandFs:  false,
		},
	}, nil
}

func (m *nfsMounter) Disconnect(ctx context.Context, info mounts.MounterContext) error {
	// unmount is managed externally
	return nil
}

func (m *nfsMounter) Expand(ctx context.Context, info mounts.MounterContext) error {
	// nfs volumes don't need expanding
	return nil
}

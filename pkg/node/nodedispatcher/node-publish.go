package nodedispatcher

import (
	"context"
	"fmt"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

func (n *Node) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target path is empty")
	}
	if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is empty")
	}
	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability missing")
	}

	// logger := logs.FromCtx(ctx)
	conf := FromCtx(ctx)
	tr := localexec.New(conf.ExecConfig)
	mp := conf.GetMultipath(tr)

	nvi, err := ParseNodeVolumeInfo(ctx, req.StagingTargetPath, tr, mp, n.conf.DevFolder, conf.FindConnectorById)
	if err != nil {
		return nil, fmt.Errorf("ParseMountInfo: %w", err)
	}

	caps, err := nvi.connector.Capabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("get connector capabilities: %w", err)
	}

	mountParams := req.VolumeCapability.GetMount()

	switch caps.ConnectType {
	case connectors.ConnectTypeBlock, connectors.ConnectTypeFolder:
		switch {
		case mountParams != nil:
			if nvi.FsMountPath == "" {
				return nil, fmt.Errorf("node volume info: FsMountPath is empty")
			}
			fsnip := fsutils.New(tr)
			var bindOpts []fsutils.BindMountOption
			// if nvi.ManagedOps.AllowIdMap {
			// 	bindOpts = append(bindOpts, fsutils.WithMapUsersDefault(), fsutils.WithMapGroup("0", "0"), fsutils.WithMapGroup("10000", req.VolumeCapability.GetMount().VolumeMountGroup))
			// }
			err = fsnip.EnsureBindMount(ctx, nvi.FsMountPath, req.TargetPath, bindOpts...)
			if err != nil {
				return nil, fmt.Errorf("EnsureBindMount: %w", err)
			}
			logs.FromCtx(ctx).MicroOp("connector.Connect: success")
		case req.VolumeCapability.GetBlock() != nil:
			return nil, fmt.Errorf("block mounting is not implemented")
		default:
			return nil, fmt.Errorf("unknown mount mode")
		}

	case connectors.ConnectTypePublishFolder:
		if mountParams == nil {
			return nil, fmt.Errorf("connector provides folder mounts, but mount params are not set")
		}

		cr := connectors.ConnectFolderRequest{
			FsMountPath: req.TargetPath,
			FsType:      mountParams.FsType,
			MountFlags:  mountParams.MountFlags,
			MountGroup:  mountParams.VolumeMountGroup,
		}

		err = nvi.publishFolderMount(ctx, cr)
		if err != nil {
			return nil, fmt.Errorf("publishFolderMount: %w", err)
		}
		logs.FromCtx(ctx).MicroOp("publishFolderMount: success")

	default:
		return nil, fmt.Errorf("unknown capabilities.ConnectType: '%s'", caps.ConnectType)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (n *Node) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target path is empty")
	}

	// logger := logs.FromCtx(ctx)
	conf := FromCtx(ctx)
	tr := localexec.New(conf.ExecConfig)
	// mp := conf.getMultipath(tr)

	fsnip := fsutils.New(tr)
	err := fsnip.Unmount(ctx, req.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("Unmount: %w", err)
	}
	err = os.Remove(req.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("remove publish dir: %w", err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

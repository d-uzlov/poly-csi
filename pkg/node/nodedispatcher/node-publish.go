package nodedispatcher

import (
	"context"
	"fmt"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meoe.io/poly-csi/pkg/node/filecon"
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

	nvi, err := TryParseMountInfo(ctx, req.StagingTargetPath, tr, conf.DevFolder, conf.BlockConnectors, conf.FsConnectors)
	if err != nil {
		return nil, fmt.Errorf("ParseMountInfo: %w", err)
	}

	mountParams := req.VolumeCapability.GetMount()
	if nvi.fsHandler == nil {
		return nil, fmt.Errorf("fs handler is not initialized")
	}

	err = nvi.fsHandler.Mount(ctx, filecon.MountRequest{
		StagingPath: nvi.FsStagePath,
		TargetPath:  req.TargetPath,
		Flags:       mountParams.MountFlags,
		FsGroup:     mountParams.VolumeMountGroup,
	})
	if err != nil {
		return nil, fmt.Errorf("fsHandler.Mount: %w", err)
	}
	// TODO move this inside filesystem?
	if mountParams.VolumeMountGroup != "" {
		if nvi.VolumeGroup != "" && nvi.VolumeGroup != mountParams.VolumeMountGroup {
			return nil, fmt.Errorf("requested to mount as group '%s' but this volume is already attached with group '%s'", mountParams.VolumeMountGroup, nvi.VolumeGroup)
		}
		err = tr.Cmds().Chown(ctx, nvi.FsStagePath, "root", mountParams.VolumeMountGroup, false)
		if err != nil {
			return nil, fmt.Errorf("chown fs root: %w", err)
		}
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

package nodedispatcher

import (
	"context"
	"fmt"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

	nvi, err := ParseNodeVolumeInfo(req.StagingTargetPath)
	if err != nil {
		return nil, fmt.Errorf("ParseMountInfo: %w", err)
	}
	if nvi.FolderPath == "" {
		return nil, fmt.Errorf("node volume info: FolderPath is empty")
	}

	fsnip := fsutils.New(n.getTransport(), n.conf.DevFolder)
	err = fsnip.EnsureBindMount(ctx, nvi.FolderPath, req.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("EnsureBindMount: %w", err)
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

	fsnip := fsutils.New(n.getTransport(), n.conf.DevFolder)
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

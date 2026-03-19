package nodedispatcher

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/mounts/fsutils"
)

func (n *Node) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}

	fsnip := fsutils.New(n.getTransport(), n.conf.DevFolder)

	// req.StagingTargetPath is empty in this call
	stats, err := fsnip.GetMountStats(req.VolumePath)
	if err != nil {
		return nil, fmt.Errorf("get volume stats failed: %w", err)
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Available: stats.Available,
				Used:      stats.Used,
				Total:     stats.Capacity,
				Unit:      csi.VolumeUsage_BYTES,
			}, {
				Available: stats.InodesAvailable,
				Used:      stats.InodesUsed,
				Total:     stats.InodesCapacity,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
		VolumeCondition: &csi.VolumeCondition{
			Abnormal: false,
			Message:  "healthy",
		},
	}, nil
}

func (n *Node) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.CapacityRange == nil {
		return nil, status.Error(codes.InvalidArgument, "capacity missing")
	}

	nvi, err := ParseNodeVolumeInfo(req.StagingTargetPath)
	if err != nil {
		return nil, fmt.Errorf("ParseMountInfo: %w", err)
	}
	if nvi.FolderPath == "" {
		return nil, fmt.Errorf("node volume info: FolderPath is empty")
	}

	connector := n.findConnectorById(nvi.ConnectorId)
	if connector == nil {
		return nil, fmt.Errorf("connector with id '%s' is not found", nvi.ConnectorId)
	}
	err = connector.Expand(ctx, nvi.SerializedConnectorData)
	if err != nil {
		return nil, fmt.Errorf("connector.Expand: %w", err)
	}
	logs.FromCtx(ctx).MicroOp("connector.Expand: success")
	if nvi.SerializedMultipathData != "" {
		err = n.getMultipath().Expand(ctx, nvi.SerializedMultipathData)
		if err != nil {
			return nil, fmt.Errorf("multipath.Expand: %w", err)
		}
		logs.FromCtx(ctx).MicroOp("multipath.Expand: success")
	}

	fsnip := fsutils.New(n.getTransport(), n.conf.DevFolder)

	if nvi.ManagedOps.ExpandFs {
		err = fsnip.ExpandFs(ctx, nvi.FolderPath)
		if err != nil {
			return nil, fmt.Errorf("ExpandFs: %w", err)
		}
		logs.FromCtx(ctx).MicroOp("ExpandFs: success")
	}

	stats, err := fsnip.GetMountStats(nvi.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("get volume stats failed: %w", err)
	}
	return &csi.NodeExpandVolumeResponse{
		CapacityBytes: stats.Capacity,
	}, nil
}

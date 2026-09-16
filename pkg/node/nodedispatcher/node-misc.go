package nodedispatcher

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

func (n *Node) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.VolumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "volume path is empty")
	}

	// logger := logs.FromCtx(ctx)
	conf := FromCtx(ctx)
	tr := localexec.New(conf.ExecConfig)
	// mp := conf.getMultipath(tr)

	fsnip := fsutils.New(tr)

	// req.StagingTargetPath is empty in this call
	stats, err := fsnip.GetMountStats(req.VolumePath)
	if err != nil {
		return nil, fmt.Errorf("GetMountStats: %w", err)
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
	}, nil
}

func (n *Node) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.VolumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "volume path is empty")
	}
	if req.CapacityRange == nil {
		return nil, status.Error(codes.InvalidArgument, "capacity missing")
	}

	// logger := logs.FromCtx(ctx)
	conf := FromCtx(ctx)
	tr := localexec.New(conf.ExecConfig)
	mp := conf.GetMultipath(tr)

	nvi, err := ParseNodeVolumeInfo(ctx, req.StagingTargetPath, tr, mp, n.conf.DevFolder, conf.FindConnectorById)
	if err != nil {
		return nil, fmt.Errorf("ParseMountInfo: %w", err)
	}

	if nvi.SerializedConnectorState != "" {
		err = nvi.conState.Expand(ctx)
		if err != nil {
			return nil, fmt.Errorf("connector.Expand: %w", err)
		}
		logs.FromCtx(ctx).MicroOp("connector.Expand: success")
	}
	if nvi.SerializedMultipathData != "" {
		err = mp.Expand(ctx, nvi.SerializedMultipathData)
		if err != nil {
			return nil, fmt.Errorf("multipath.Expand: %w", err)
		}
		logs.FromCtx(ctx).MicroOp("multipath.Expand: success")
	}

	fsnip := fsutils.New(tr)

	// Dispatcher expand manages filesystems mounted from block devices.
	// Other cases should be handled in connectors.
	if nvi.MountType == mountTypeBlock {
		err = fsnip.ExpandFs(ctx, nvi.FsMountPath, conf.DevFolder)
		if err != nil {
			return nil, fmt.Errorf("ExpandFs: %w", err)
		}
		logs.FromCtx(ctx).MicroOp("ExpandFs: success")
	}

	stats, err := fsnip.GetMountStats(req.VolumePath)
	if err != nil {
		return nil, fmt.Errorf("GetMountStats: %w", err)
	}
	return &csi.NodeExpandVolumeResponse{
		CapacityBytes: stats.Capacity,
	}, nil
}

func (n *Node) NodeGetVolumeHealth(ctx context.Context, req *csi.NodeGetVolumeHealthRequest) (*csi.NodeGetVolumeHealthResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging path is empty")
	}
	conf := FromCtx(ctx)
	tr := localexec.New(conf.ExecConfig)
	mp := conf.GetMultipath(tr)

	nvi, err := ParseNodeVolumeInfo(ctx, req.StagingTargetPath, tr, mp, n.conf.DevFolder, conf.FindConnectorById)
	if err != nil {
		return nil, fmt.Errorf("ParseMountInfo: %w", err)
	}

	if nvi.conState == nil {
		return nil, fmt.Errorf("connection state is not initialized")
	}
	issues, err := nvi.conState.CheckHealth(ctx)
	if err != nil {
		return nil, fmt.Errorf("CheckHealth: %w", err)
	}

	result := &csi.NodeGetVolumeHealthResponse{
		VolumeHealth: &csi.VolumeHealth{
			VolumeId: req.VolumeId,
		},
	}

	for _, v := range issues {
		if len(v.Reason) > 100 {
			v.Message = v.Message[:100]
		}
		if len(v.Message) > 500 {
			v.Message = v.Message[:500]
		}
		result.VolumeHealth.HealthStatuses = append(result.VolumeHealth.HealthStatuses, &csi.VolumeHealth_VolumeHealthEntry{
			Status:  convertHealthStatus(v.Status),
			Reason:  v.Reason,
			Message: v.Message,
		})
	}

	return result, nil
}

func convertHealthStatus(connectorStatus string) csi.VolumeHealthErrorType {
	switch connectorStatus {
	case connectors.HealthStatusDegraded:
		return csi.VolumeHealthErrorType_DEGRADED
	case connectors.HealthStatusDataLoss:
		return csi.VolumeHealthErrorType_DATA_LOSS
	case connectors.HealthStatusInaccessible:
		return csi.VolumeHealthErrorType_INACCESSIBLE
	}
	return csi.VolumeHealthErrorType_DEGRADED
}

package nodedispatcher

import (
	"context"
	"fmt"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meoe.io/poly-csi/pkg/controller/volumecontext"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/addropt"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/encrypt"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
	"meoe.io/poly-csi/pkg/utils/volumeinit"
)

const stageFolderMount = "/fs_mount"

func (n *Node) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (res *csi.NodeStageVolumeResponse, err error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is empty")
	}
	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability missing")
	}

	logger := logs.FromCtx(ctx)
	conf := FromCtx(ctx)
	tr := localexec.New(conf.ExecConfig)
	mp := conf.GetMultipath(tr)

	var vc volumecontext.VolumeContext
	err = serialmaps.Parse(req.VolumeContext, volumecontext.Prefix, &vc)
	if err != nil {
		return nil, fmt.Errorf("parse volume context: %w", err)
	}

	connector := conf.FindConnectorById(vc.ConnectorId, tr)
	if connector == nil {
		return nil, fmt.Errorf("connector with id '%s' is not found", vc.ConnectorId)
	}

	caps, err := connector.Capabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("get connector capabilities: %w", err)
	}

	nvi := newVolumeHandle(req.StagingTargetPath, tr, connector, mp, vc.ConnectorId)
	defer func() {
		saveErr := nvi.writeToDisk()
		if err == nil {
			err = saveErr
			saveErr = nil
		}
		if saveErr != nil {
			logger.Error("failed to write volume metadata to disk", "error", saveErr.Error())
		}
		if err != nil {
			if conf.DisableCleanupOnFailure {
				logger.Warn("detected failure, but skipping cleanup due to config")
				return
			}
			logger.Warn("detected failure, running cleanup")
			cleanupErr := nvi.cleanup(ctx)
			if cleanupErr != nil {
				// TODO retry cleanup in background?
				logger.Error("failed to cleanup partially initialized volume", "error", cleanupErr.Error())
			}
		}
	}()

	secretPlaintext, err := encrypt.ExtractSecretFromMap(req.PublishContext, conf.Encrypt.NodeKey)
	if err != nil {
		return nil, fmt.Errorf("ExtractSecretFromMap: %w", err)
	}

	addressOptions, err := addropt.Parse(req.PublishContext)
	if err != nil {
		return nil, fmt.Errorf("parse address options from publish context: %w", err)
	}

	mountParams := req.VolumeCapability.GetMount()

	switch caps.ConnectType {
	case connectors.ConnectTypeBlock:
		err = nvi.stageBlock(ctx, connectors.ConnectRequest{
			FsMountPath:             "",
			AddressOptions:          addressOptions,
			VolumeContext:           req.VolumeContext,
			PublishContext:          req.PublishContext,
			AccessMount:             nil,
			PublishSecret:           secretPlaintext,
			DisableCleanupOnFailure: conf.DisableCleanupOnFailure,
		})
		if err != nil {
			return nil, fmt.Errorf("stageBlock: %w", err)
		}
		switch {
		case mountParams != nil:
			var initConfig volumeinit.InitConfig
			err = serialmaps.Parse(req.PublishContext, volumeinit.PreferredPrefix, &initConfig)
			if err != nil {
				return nil, fmt.Errorf("parse init config: %w", err)
			}
			err = nvi.stageBlockMount(ctx, req.StagingTargetPath+stageFolderMount, mountParams.MountFlags, initConfig)
			if err != nil {
				return nil, fmt.Errorf("stageBlockMount: %w", err)
			}
			logger.MicroOp("stageBlockMount: success")
		case req.VolumeCapability.GetBlock() != nil:
			return nil, fmt.Errorf("block mounting is not implemented")
		default:
			return nil, fmt.Errorf("unknown mount mode")
		}

	case connectors.ConnectTypeFolder:
		if mountParams == nil {
			return nil, fmt.Errorf("connector provides folder mounts, but mount params are not set")
		}

		err = nvi.stageFolderMount(ctx, connectors.ConnectRequest{
			FsMountPath:             req.StagingTargetPath + stageFolderMount,
			AddressOptions:          addressOptions,
			VolumeContext:           req.VolumeContext,
			PublishContext:          req.PublishContext,
			AccessMount:             mountParams,
			PublishSecret:           secretPlaintext,
			DisableCleanupOnFailure: conf.DisableCleanupOnFailure,
		})
		if err != nil {
			return nil, fmt.Errorf("stageFolderMount: %w", err)
		}
		logs.FromCtx(ctx).MicroOp("stageFolderMount: success")
	case connectors.ConnectTypePublishFolder:
		if mountParams == nil {
			return nil, fmt.Errorf("connector provides folder mounts, but mount params are not set")
		}
		// nothing to do at stage
	default:
		return nil, fmt.Errorf("unknown capabilities.ConnectType: '%s'", caps.ConnectType)
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (n *Node) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (res *csi.NodeUnstageVolumeResponse, err error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is empty")
	}

	// logger := logs.FromCtx(ctx)
	conf := FromCtx(ctx)
	tr := localexec.New(conf.ExecConfig)
	mp := conf.GetMultipath(tr)

	nvi, err := TryParseMountInfo(req.StagingTargetPath, tr, mp, func(connectorId string) connectors.Connector { return conf.FindConnectorById(connectorId, tr) })
	if err != nil {
		return nil, fmt.Errorf("ParseMountInfo: %w", err)
	}
	if nvi != nil {
		err = nvi.cleanup(ctx)
		if err != nil {
			return nil, fmt.Errorf("cleanup: %w", err)
		}
	}

	err = os.Remove(req.StagingTargetPath)
	if err != nil {
		return nil, fmt.Errorf("remove staging dir: %w", err)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

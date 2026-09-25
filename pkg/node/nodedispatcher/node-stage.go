package nodedispatcher

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meoe.io/poly-csi/pkg/controller/volumecontext"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/addropt"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/nodeconfig"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/blockdevice"
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

	fullPreset := map[string]string{}
	if vc.PresetName != "" {
		for v := range strings.SplitSeq(vc.PresetName, ",") {
			preset, ok := n.conf.Presets[v]
			if !ok {
				return nil, fmt.Errorf("preset with name '%s' is not found", v)
			}
			maps.Copy(fullPreset, preset)
		}
	}
	logs.FromCtx(ctx).MicroOp("using preset", "fullPreset", fullPreset)

	conTemplate := conf.FindConnectorById(vc.ConnectorId)
	if conTemplate == nil {
		return nil, fmt.Errorf("connector with id '%s' is not found", vc.ConnectorId)
	}

	connector := conTemplate.Inflate()
	if connector == nil {
		return nil, fmt.Errorf("createConnector 'id=%s': inflate returned nil", vc.ConnectorId)
	}

	err = connector.Configure(ctx, connectors.CommonConfig{
		Tr:        tr,
		DevFolder: n.conf.DevFolder,
	}, conTemplate.Conf.ByteConfig)
	if err != nil {
		return nil, fmt.Errorf("connector.Configure: %w", err)
	}

	caps, err := connector.Capabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("get connector capabilities: %w", err)
	}

	secretPlaintext, err := encrypt.ExtractSecretFromMap(req.PublishContext, conf.Encrypt.NodeKey)
	if err != nil {
		return nil, fmt.Errorf("ExtractSecretFromMap: %w", err)
	}

	addressOptions, err := addropt.Parse(req.PublishContext)
	if err != nil {
		return nil, fmt.Errorf("parse address options from publish context: %w", err)
	}

	connectorContext := map[string]string{}
	for k, v := range req.PublishContext {
		if strings.HasPrefix(k, caps.ContextPrefix) {
			connectorContext[k] = v
		}
	}

	ch, err := connector.GetHandle(ctx, connectors.ConnectRequest{
		AddressOptions:          addressOptions,
		VolumeContext:           req.VolumeContext,
		PublishContext:          connectorContext,
		PublishSecret:           secretPlaintext,
		DisableCleanupOnFailure: conf.DisableCleanupOnFailure,
		Preset:                  fullPreset,
	})
	if err != nil {
		return nil, fmt.Errorf("connector.GetHandle: %w", err)
	}
	if ch == nil {
		return nil, fmt.Errorf("connector.GetHandle: nil result")
	}

	nvi := newVolumeHandle(req.StagingTargetPath, tr, connector, mp, vc.ConnectorId, fullPreset)
	nvi.conState = ch
	nvi.FsRootDirMode = vc.FsRootDirMode
	nvi.JournalVolume = vc.JournalVolume
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

	mountParams := req.VolumeCapability.GetMount()

	switch caps.ConnectType {
	case connectors.ConnectTypeBlock:
		err = nvi.stageBlock(ctx)
		if err != nil {
			return nil, fmt.Errorf("stageBlock: %w", err)
		}
		switch {
		case mountParams != nil:
			nvi.FsType = vc.FsType
			nvi.fs = volumeinit.GetFromType(vc.FsType)
			err = nvi.fs.Inflate(vc.FsData)
			if err != nil {
				return nil, fmt.Errorf("fs.Inflate: %w", err)
			}
			err = nvi.stageBlockMount(ctx, req.StagingTargetPath+stageFolderMount, mountParams.MountFlags)
			if err != nil {
				return nil, fmt.Errorf("stageBlockMount: %w", err)
			}
			logger.MicroOp("stageBlockMount: success")
		case req.VolumeCapability.GetBlock() != nil:
			return nil, fmt.Errorf("block mounting is not implemented")
		default:
			return nil, fmt.Errorf("unknown mount mode")
		}
		if n.conf.PodCgroupRoot != "" {
			dev, err := nvi.getMainBlockDevice(ctx)
			if err != nil {
				return nil, fmt.Errorf("getMainBlockDevice: %w", err)
			}

			limits, foundAny, err := nodeconfig.ParseCgroupIoLimits(fullPreset)
			if err != nil {
				return nil, fmt.Errorf("ParseCgroupIoLimits: %w", err)
			}
			if foundAny {
				logs.FromCtx(ctx).MicroOp("configuring cgroup io.max for volume", "limits", limits)
				blockPath, err := tr.Cmds().ReadLink(ctx, dev)
				if err != nil {
					return nil, fmt.Errorf("readLink(%s): %w", dev, err)
				}
				bsnip := blockdevice.NewSnippets(tr, filepath.Base(blockPath))
				// TODO this also affects csi-driver itself
				err = bsnip.SetCgroupsIoLimit(ctx, n.conf.PodCgroupRoot, limits)
				if err != nil {
					return nil, fmt.Errorf("SetCgroupsIoLimit: %w", err)
				}
			}
		}

	case connectors.ConnectTypeFolder:
		if mountParams == nil {
			return nil, fmt.Errorf("connector provides folder mounts, but mount params are not set")
		}

		cr := connectors.ConnectFolderRequest{
			FsMountPath: req.StagingTargetPath + stageFolderMount,
			FsType:      mountParams.FsType,
			MountFlags:  mountParams.MountFlags,
			// MountGroup:  mountParams.VolumeMountGroup,
		}

		err = nvi.stageFolderMount(ctx, cr)
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

	nvi, err := TryParseMountInfo(ctx, req.StagingTargetPath, tr, mp, n.conf.DevFolder, conf.FindConnectorById)
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

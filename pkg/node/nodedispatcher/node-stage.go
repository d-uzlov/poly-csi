package nodedispatcher

import (
	"context"
	"fmt"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meoe.io/poly-csi/pkg/api/publishcontext"
	"meoe.io/poly-csi/pkg/api/volumecontext"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/blockcon"
	"meoe.io/poly-csi/pkg/node/filecon"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
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

	var vc volumecontext.VolumeContext
	err = serialmaps.Parse(req.VolumeContext, volumecontext.Prefix, &vc)
	if err != nil {
		return nil, fmt.Errorf("parse volume context: %w", err)
	}

	fullPreset, err := conf.AssemblePreset(vc.Presets)
	if err != nil {
		return nil, fmt.Errorf("parse preset: %w", err)
	}
	logs.FromCtx(ctx).MicroOp("using preset", "fullPreset", fullPreset)

	pc, err := publishcontext.Parse(req.PublishContext, conf.Encrypt.NodeKey)
	if err != nil {
		return nil, fmt.Errorf("parse publish context: %w", err)
	}
	logger.MicroOp("parsed publish context", "publish-context", pc)
	// err = pc.Validate()
	// if err != nil {
	// 	return nil, fmt.Errorf("validateContext: %w", err)
	// }

	nvi := newVolumeHandle(req.StagingTargetPath, tr, fullPreset)
	nvi.FsRootDirMode = vc.FsRootDirMode
	defer func() {
		logger.Error("writing volume info to disk", "info", nvi)
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

	devices := map[string]string{}
	for k, v := range pc.BlockDevices {
		con := conf.BlockConnectors[v.ConnectorId]
		if con == nil {
			return nil, fmt.Errorf("block connector with id '%s' is not found", v.ConnectorId)
		}

		handle, err := con.GetHandle(ctx, blockcon.ConnectRequest{
			Uuid:                    vc.Uuid,
			Proto:                   v.Data,
			DisableCleanupOnFailure: conf.DisableCleanupOnFailure,
			Preset:                  fullPreset,
		})
		if err != nil {
			return nil, fmt.Errorf("blockcon.GetHandle: %w", err)
		}
		if handle == nil {
			return nil, fmt.Errorf("blockcon.GetHandle: nil result")
		}

		err = handle.Connect(ctx)
		if err != nil {
			return nil, fmt.Errorf("blockcon.Connect: %w", err)
		}

		dd, err := handle.GetDevices(ctx)
		if err != nil {
			return nil, fmt.Errorf("blockcon.GetDevices: %w", err)
		}

		// var devicePath string
		// devicePath = setupMultipath(dd)
		// devices[k] = devicePath
		devices[k] = dd[0]
		nvi.Blocks[k] = Block{
			ConnectorId: v.ConnectorId,
			handler:     handle,
		}
	}

	mountParams := req.VolumeCapability.GetMount()

	fs, ok := conf.FsConnectors[vc.FsType]
	if !ok {
		return nil, fmt.Errorf("filesystem is not found: '%s'", vc.FsType)
	}
	fsHandle, err := fs.GetHandle(ctx, filecon.ConnectRequest{
		Uuid:                    vc.Uuid,
		Proto:                   pc.Filesystem,
		DisableCleanupOnFailure: conf.DisableCleanupOnFailure,
		Preset:                  fullPreset,
	})
	if err != nil {
		return nil, fmt.Errorf("filecon.GetHandle: %w", err)
	}
	if fsHandle == nil {
		return nil, fmt.Errorf("filecon.GetHandle: nil result")
	}
	nvi.FsType = vc.FsType
	nvi.fsHandler = fsHandle

	nvi.FsStagePath = req.StagingTargetPath + stageFolderMount

	err = fsHandle.Prepare(ctx, filecon.PrepareRequest{
		Devices:     devices,
		StagingPath: nvi.FsStagePath,
		Flags:       mountParams.MountFlags,
	})
	if err != nil {
		return nil, fmt.Errorf("fsHandle.Prepare: %w", err)
	}

	cmd := []string{"chmod", vc.FsRootDirMode, nvi.FsStagePath}
	err = transport.CheckedRun(ctx, tr, cmd, transport.WithSudo(true))
	if err != nil {
		return nil, fmt.Errorf("chmod: %w", err)
	}

	// TODO
	// if n.conf.PodCgroupRoot != "" {
	// 	dev, err := nvi.getMainBlockDevice(ctx)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("getMainBlockDevice: %w", err)
	// 	}
	// 	limits, foundAny, err := nodeconfig.ParseCgroupIoLimits(fullPreset)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("ParseCgroupIoLimits: %w", err)
	// 	}
	// 	if foundAny {
	// 		logs.FromCtx(ctx).MicroOp("configuring cgroup io.max for volume", "limits", limits)
	// 		blockPath, err := tr.Cmds().ReadLink(ctx, dev)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("readLink(%s): %w", dev, err)
	// 		}
	// 		bsnip := blockdevice.NewSnippets(tr, filepath.Base(blockPath))
	// 		// TODO this also affects csi-driver itself
	// 		err = bsnip.SetCgroupsIoLimit(ctx, n.conf.PodCgroupRoot, limits)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("SetCgroupsIoLimit: %w", err)
	// 		}
	// 	}
	// }

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

	nvi, err := TryParseMountInfo(ctx, req.StagingTargetPath, tr, conf.DevFolder, conf.BlockConnectors, conf.FsConnectors)
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

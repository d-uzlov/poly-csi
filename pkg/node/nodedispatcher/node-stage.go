package nodedispatcher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meoe.io/poly-csi/pkg/node/addropt"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

func (n *Node) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (res *csi.NodeStageVolumeResponse, err error) {
	logger := slogctx.FromCtx(ctx)

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is empty")
	}
	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability missing")
	}

	mounterId, ok := req.VolumeContext[mounts.MountIdKey]
	if !ok {
		return nil, fmt.Errorf("volume context: '%s' missing", mounts.MountIdKey)
	}

	pcSecretEnc := req.PublishContext["_secret"]
	var pcSecret map[string]string
	if pcSecretEnc != "" {
		aesSecret, err := encrypt.ExtractAesSecret(req.PublishContext, n.getPrivateKey())
		if err != nil {
			return nil, fmt.Errorf("publish_context: extract aes secret: %w", err)
		}
		ciphertext, err := base64.StdEncoding.DecodeString(pcSecretEnc)
		if err != nil {
			return nil, fmt.Errorf("publish_context: extract secret value: %w", err)
		}
		plaintext, err := encrypt.DecryptAes(aesSecret, ciphertext)
		err = json.Unmarshal(plaintext, &pcSecret)
		if err != nil {
			return nil, fmt.Errorf("publish_context: unmarshal decrypted secret value: %w", err)
		}
	}

	mounter := n.findMounterById(mounterId)
	if mounter == nil {
		return nil, fmt.Errorf("mounter with id '%s' is not found", mounterId)
	}

	nvi := NodeVolumeInfo{}
	nvi.MounterId = mounterId
	nvi.MounterPath = req.StagingTargetPath + "/mounter"

	err = os.MkdirAll(nvi.MounterPath, 0777)
	if err != nil {
		return nil, fmt.Errorf("create '/mounter' path for staging")
	}

	// At this point we have modified staging path (at the very least we have created nvi.MounterPath)
	// so we can later receive Unstage call.
	// Even if NodeVolumeInfo is only partially complete, we still want to save it, so we can use it in Unstage.
	defer func() {
		err2 := SaveMountInfo(req.StagingTargetPath, nvi)
		if err2 == nil {
			return
		}
		if err == nil {
			err = fmt.Errorf("SaveMountInfo: %w", err2)
			return
		}
		slogctx.FromCtx(ctx).Error("failed to save partial volume info", "error", err2)
	}()

	addressOptions, err := addropt.Parse(req.PublishContext)
	if err != nil {
		return nil, fmt.Errorf("parse address options from publish context: %w", err)
	}

	cr, err := mounter.Connect(ctx, mounts.ConnectRequest{
		AddressOptions:       addressOptions,
		VolumeContext:        req.VolumeContext,
		PublishContext:       req.PublishContext,
		StagingTargetPath:    nvi.MounterPath,
		AccessBlock:          req.VolumeCapability.GetBlock(),
		AccessMount:          req.VolumeCapability.GetMount(),
		Secrets:              req.Secrets,
		PublishContextSecret: pcSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("mounter[id=%s].Connect: %w", mounterId, err)
	}
	if cr == nil {
		return nil, fmt.Errorf("mounter[id=%s].Connect: did not set mounter context", mounterId)
	}
	if cr.MounterData != nil {
		if cr.SerializedMounterData != "" {
			return nil, fmt.Errorf("mounter set both MounterData and SerializedMounterData")
		}
		encoded, err := json.Marshal(cr.MounterData)
		if err != nil {
			return nil, fmt.Errorf("serialize MounterData: %w", err)
		}
		nvi.SerializedMounterData = string(encoded)
	} else {
		nvi.SerializedMounterData = cr.SerializedMounterData
	}
	logger.Info("mounter.Connect: success")

	nvi.ManagedOps = cr.ManagedOps

	if cr.FolderPath != "" {
		nvi.FolderPath = cr.FolderPath
		// folder path is already set, nothing to do here
		logger.Info("mounter has set folder path, skipping ManagedOps")
		return &csi.NodeStageVolumeResponse{}, nil
	}
	if !nvi.ManagedOps.MountFs {
		return nil, fmt.Errorf("mounter[id=%s]: FolderPath is empty", mounterId)
	}

	// ensureDevicePath will setup multipath if needed
	device, err := ensureDevicePath(ctx, &nvi, cr.Devices, n.getMultipath())
	if err != nil {
		return nil, fmt.Errorf("ensureDevicePath: %w", err)
	}
	logger.Info("ManagedOps: got device path")

	requestMount := req.VolumeCapability.GetMount()
	if requestMount == nil {
		return nil, fmt.Errorf("block mounting is not implemented")
	}
	err = ensureFolderPath(ctx, device, &nvi, requestMount, req.StagingTargetPath+"/fs_mount")
	if err != nil {
		return nil, fmt.Errorf("ensureFolderPath: %w", err)
	}
	logger.Info("ManagedOps: mounted /fs_mount")

	return &csi.NodeStageVolumeResponse{}, nil
}

func (n *Node) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (res *csi.NodeUnstageVolumeResponse, err error) {
	logger := slogctx.FromCtx(ctx)

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is empty")
	}

	nvi, err := ParseNodeVolumeInfo(req.StagingTargetPath)
	if err != nil {
		// volume info is missing, but CO thinks volume is still staged
		// let's try to remove staging path, but only if it's an empty folder
		// This way we can at least allow cluster administrator to manually cleanup all related resources
		err = os.Remove(req.StagingTargetPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("volume is stuck: local info is missing but staging path is not empty: %s", req.StagingTargetPath)
		}
		return &csi.NodeUnstageVolumeResponse{}, nil
	}
	if nvi == nil {
		return nil, fmt.Errorf("node volume info is not found")
	}

	if nvi.ManagedOps.UnmountFs {
		err = ensureFolderUnmounted(nvi)
		if err != nil {
			return nil, fmt.Errorf("ensureFolderUnmounted: %w", err)
		}
		logger.Info("ManagedOps: successfully unmounted folder")
	}

	err = ensureMultipathDisabled(ctx, nvi, n.getMultipath())
	if err != nil {
		return nil, fmt.Errorf("ensureMultipathDisabled: %w", err)
	}
	logger.Info("ManagedOps: multipath disabled OK")

	mounter := n.findMounterById(nvi.MounterId)
	if mounter == nil {
		return nil, fmt.Errorf("mounter with id '%s' is not found", nvi.MounterId)
	}
	err = mounter.Disconnect(ctx, nvi.SerializedMounterData)
	if err != nil {
		return nil, fmt.Errorf("mounter.Disconnect: %w", err)
	}
	logger.Info("mounter.Disconnect: success")
	// Try to remove mounted folder path.
	// This way mounters don't have to implement it.
	// Additional benefit is that we ensure that volume is not mounted anymore.
	if nvi.FolderPath != "" {
		err = os.Remove(nvi.FolderPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("remove mounted volume folder: %w", err)
		}
	}

	err = os.Remove(nvi.MounterPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("remove mounter staging dir: %w", err)
	}
	dirEntries, err := os.ReadDir(req.StagingTargetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			slogctx.FromCtx(ctx).Warn("staging path unexpectedly disappeared")
			return &csi.NodeUnstageVolumeResponse{}, nil
		}
		return nil, err
	}
	if len(dirEntries) != 1 {
		return nil, fmt.Errorf("staging path contains unexpected files")
	}
	err = DeleteMountInfo(req.StagingTargetPath)
	if err != nil {
		return nil, fmt.Errorf("DeleteMountInfo: %w", err)
	}
	// It's a dangerous place to call os.Remove.
	// We have already deleted mount info, so if we fail, the volume will become stuck.
	// But the only way for this to happen is if a new file was created between os.ReadDir and here.
	// This is an exceptional situation, and node administrator must manage it manually.
	err = os.Remove(req.StagingTargetPath)
	if err != nil {
		return nil, fmt.Errorf("remove staging dir: %w", err)
	}
	return &csi.NodeUnstageVolumeResponse{}, nil
}

package node

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"meoe.io/poly-csi/pkg/node/addropt"
	"meoe.io/poly-csi/pkg/node/mountinfo"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/fsutils"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/node/nodeinfo"
	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

func CreateNode(logger *slog.Logger, name string, version string, nodeId string) (*Node, error) {
	return &Node{
		name:    name,
		version: version,
		nodeId:  nodeId,
	}, nil
}

type Node struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedNodeServer

	name    string
	version string
	nodeId  string

	confMutex sync.RWMutex
	conf      parsedConfig

	nodeInfoCancel func()
	nodeInfoCh     <-chan struct{}
}

func (n *Node) ReloadConfig(ctx context.Context, newConf parsedConfig) error {
	if n.nodeInfoCh != nil && n.nodeInfoCancel != nil {
		n.nodeInfoCancel()
		<-n.nodeInfoCh
		n.nodeInfoCancel = nil
		n.nodeInfoCh = nil
	}

	tr, err := localexec.New(ctx)
	if err != nil {
		panic(err.Error())
	}

	mounts := []mounts.NodeMounter{}
	for _, v := range newConf.Mounts {
		if !v.Valid {
			continue
		}
		m := v.Inflate()
		if m == nil {
			continue
		}
		mounts = append(mounts, m)
	}
	var mp multipath.Multipath
	if newConf.Multipath.Valid {
		mp = newConf.Multipath.Inflate()
	} else {
		slogctx.FromCtx(ctx).Error("multipath config is not valid, disabling multipath")
	}

	aesSecret := make([]byte, 32)
	_, err = rand.Read(aesSecret)
	if err != nil {
		return fmt.Errorf("generate random bytes for aes: %w", err)
	}

	pushCtx, pushCancel := context.WithCancel(ctx)
	pushCtx = transport.MakeContext(pushCtx, tr)
	pushCh := make(chan struct{})

	nodeinfo.SetupPush(pushCtx, newConf.NodeInfoDb.Address, newConf.NodeInfoDb.Interval, func() regnodeinfo.NodeInfo {
		return nodeinfo.GatherNodeInfo(pushCtx, n.nodeId, newConf.Encrypt.NodeKey, mounts, aesSecret, newConf.Encrypt.ControllerKey, mp, newConf.Networks)
	}, func() {})

	n.nodeInfoCancel = pushCancel
	n.nodeInfoCh = pushCh

	n.confMutex.Lock()
	defer n.confMutex.Unlock()

	n.conf = newConf
	return nil
}

func (n *Node) findMounterById(id string) mounts.NodeMounter {
	n.confMutex.RLock()
	defer n.confMutex.RUnlock()

	res := n.conf.findMounterById(id)
	if res == nil {
		return nil
	}
	return res.Inflate()
}

func (n *Node) getMultipath() multipath.Multipath {
	n.confMutex.RLock()
	defer n.confMutex.RUnlock()

	if !n.conf.Multipath.Valid {
		return nil
	}
	if n.conf.Multipath.Inflate == nil {
		return nil
	}
	return n.conf.Multipath.Inflate()
}

func (n *Node) getPrivateKey() encrypt.RsaPrivateKey {
	n.confMutex.RLock()
	defer n.confMutex.RUnlock()

	return n.conf.Encrypt.NodeKey
}

func (n *Node) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          n.name,
		VendorVersion: n.version,
	}, nil
}

func (n *Node) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{
		Ready: wrapperspb.Bool(true),
	}, nil
}

func (n *Node) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	caps := []*csi.PluginCapability{
		{
			Type: &csi.PluginCapability_VolumeExpansion_{
				VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
					Type: csi.PluginCapability_VolumeExpansion_ONLINE,
				},
			},
		},
	}

	return &csi.GetPluginCapabilitiesResponse{Capabilities: caps}, nil
}

func (n *Node) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	resp := &csi.NodeGetInfoResponse{
		NodeId:            n.nodeId,
		MaxVolumesPerNode: 0,
	}

	return resp, nil
}

func (n *Node) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	capsList := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
		// csi.NodeServiceCapability_RPC_VOLUME_CONDITION,
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
		// csi.NodeServiceCapability_RPC_VOLUME_MOUNT_GROUP,
	}
	result := []*csi.NodeServiceCapability{}
	for _, c := range capsList {
		result = append(result, &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: c,
				},
			},
		})
	}

	return &csi.NodeGetCapabilitiesResponse{Capabilities: result}, nil
}

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

	nvi := mountinfo.NodeVolumeInfo{}
	nvi.MounterId = mounterId
	nvi.MounterPath = req.StagingTargetPath + "/mounter"

	err = os.MkdirAll(nvi.MounterPath, 0777)
	if err != nil {
		return nil, fmt.Errorf("create '/mounter' path for staging")
	}

	// At this point we have modified staging path (at the very least we have created nvi.MounterPath)
	// so we can later receive Unstage call.
	// We want to save partially complete NodeVolumeInfo so we can use it in Unstage.
	defer func() {
		err2 := mountinfo.SaveMountInfo(req.StagingTargetPath, nvi)
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

	mc, err := mounter.Connect(ctx, mounts.ConnectRequest{
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
	if mc == nil {
		return nil, fmt.Errorf("mounter[id=%s].Connect: did not set mounter context", mounterId)
	}
	logger.Info("mounter.Connect: success")
	nvi.MounterContext = *mc

	if nvi.MounterContext.FolderPath != "" {
		nvi.FolderPath = nvi.MounterContext.FolderPath
		// folder path is already set, nothing to do here
		logger.Info("mounter has set folder path, skipping ManagedOps")
		return &csi.NodeStageVolumeResponse{}, nil
	}
	if !nvi.MounterContext.ManagedOps.MountFs {
		return nil, fmt.Errorf("mounter[id=%s]: FolderPath is empty", mounterId)
	}

	// ensureDevicePath will setup multipath if needed
	err = ensureDevicePath(ctx, &nvi, n.getMultipath())
	if err != nil {
		return nil, fmt.Errorf("ensureDevicePath: %w", err)
	}
	logger.Info("ManagedOps: got device path")

	requestMount := req.VolumeCapability.GetMount()
	if requestMount == nil {
		return nil, fmt.Errorf("block mounting is not implemented")
	}
	err = ensureFolderPath(ctx, &nvi, requestMount, req.StagingTargetPath+"/fs_mount")
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

	nvi, err := mountinfo.ParseNodeVolumeInfo(req.StagingTargetPath)
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

	if nvi.MounterContext.ManagedOps.UnmountFs {
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
	err = mounter.Disconnect(ctx, nvi.MounterContext)
	if err != nil {
		return nil, fmt.Errorf("mounter.Disconnect: %w", err)
	}
	logger.Info("mounter.Disconnect: success")
	// Try to remove mounted folder path.
	// This way mounters don't have to implement it.
	// Additional benefit is that we ensure that volume is not mounted anymore.
	if nvi.MounterContext.FolderPath != "" {
		err = os.Remove(nvi.MounterContext.FolderPath)
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
	err = mountinfo.DeleteMountInfo(req.StagingTargetPath)
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

	nvi, err := mountinfo.ParseNodeVolumeInfo(req.StagingTargetPath)
	if err != nil {
		return nil, fmt.Errorf("ParseMountInfo: %w", err)
	}
	if nvi.FolderPath == "" {
		return nil, fmt.Errorf("node volume info: FolderPath is empty")
	}
	err = fsutils.EnsureBindMount(nvi.FolderPath, req.TargetPath)
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
	err := fsutils.Unmount(ctx, req.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("Unmount: %w", err)
	}
	err = os.Remove(req.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("remove publish dir: %w", err)
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (n *Node) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}

	// req.StagingTargetPath is empty in this call
	stats, err := fsutils.GetMountStats(req.VolumePath)
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

	nvi, err := mountinfo.ParseNodeVolumeInfo(req.StagingTargetPath)
	if err != nil {
		return nil, fmt.Errorf("ParseMountInfo: %w", err)
	}
	if nvi.FolderPath == "" {
		return nil, fmt.Errorf("node volume info: FolderPath is empty")
	}

	mounter := n.findMounterById(nvi.MounterId)
	if mounter == nil {
		return nil, fmt.Errorf("mounter with id '%s' is not found", nvi.MounterId)
	}
	err = mounter.Expand(ctx, nvi.MounterContext)
	if err != nil {
		return nil, err
	}

	if nvi.MounterContext.ManagedOps.ExpandFs {
		err = fsutils.ExpandFs(ctx, nvi.FolderPath)
		if err != nil {
			return nil, fmt.Errorf("ExpandFs: %w", err)
		}
	}

	stats, err := fsutils.GetMountStats(nvi.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("get volume stats failed: %w", err)
	}
	return &csi.NodeExpandVolumeResponse{
		CapacityBytes: stats.Capacity,
	}, nil
}

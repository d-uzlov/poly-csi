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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/addropt"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

// TODO add cleanup on fail
func (n *Node) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (res *csi.NodeStageVolumeResponse, err error) {
	logger := logs.FromCtx(ctx)

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is empty")
	}
	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability missing")
	}

	connectorId, ok := req.VolumeContext[connectors.ConnectorIdKey]
	if !ok {
		return nil, fmt.Errorf("volume context: '%s' missing", connectors.ConnectorIdKey)
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

	connector := n.findConnectorById(connectorId)
	if connector == nil {
		return nil, fmt.Errorf("connector with id '%s' is not found", connectorId)
	}

	nvi := newVolumeHandle(req.StagingTargetPath)
	defer func() {
		if err != nil {
			cleanupErr := nvi.cleanup(ctx, n.getMultipath(), connector)
			if cleanupErr != nil {
				// TODO retry cleanup in background?
				logger.Error("failed to cleanup partially initialized volume", "error", err)
			}
		}
	}()

	addressOptions, err := addropt.Parse(req.PublishContext)
	if err != nil {
		return nil, fmt.Errorf("parse address options from publish context: %w", err)
	}

	err = nvi.connect(ctx, connectorId, connector, connectors.ConnectRequest{
		AddressOptions: addressOptions,
		VolumeContext:  req.VolumeContext,
		PublishContext: req.PublishContext,
		// StagingTargetPath:    "", // StagingTargetPath is set automatically
		AccessBlock:          req.VolumeCapability.GetBlock(),
		AccessMount:          req.VolumeCapability.GetMount(),
		Secrets:              req.Secrets,
		PublishContextSecret: pcSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("connect via connector[id=%s]: %w", connectorId, err)
	}
	logger.MicroOpSuccess("connector.Connect: success")

	mountRequest := req.VolumeCapability.GetMount()
	if mountRequest == nil {
		return nil, fmt.Errorf("block mounting is not implemented")
	}

	err = nvi.ensureFolderPath(ctx, n.getTransport(), n.getMultipath(), mountRequest)
	if err != nil {
		return nil, fmt.Errorf("connector[id=%s]: ensure folder path exists: %w", connectorId, err)
	}
	logger.MicroOpSuccess("ensure folder path: success")

	// nvi.Initialized = true
	err = nvi.writeMetadataToDisk()
	if err != nil {
		return nil, fmt.Errorf("writeMetadataToDisk: %w", err)
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (n *Node) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (res *csi.NodeUnstageVolumeResponse, err error) {
	// logger := slogctx.FromCtx(ctx)

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
		// TODO try to remove empty staging path before returning an error?
		return nil, fmt.Errorf("node volume info is not found")
	}

	connector := n.findConnectorById(nvi.ConnectorId)
	if connector == nil {
		return nil, fmt.Errorf("connector with id '%s' is not found", nvi.ConnectorId)
	}

	err = nvi.cleanup(ctx, n.getMultipath(), connector)
	if err != nil {
		return nil, fmt.Errorf("cleanup: %w", err)
	}

	dirEntries, err := os.ReadDir(req.StagingTargetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logs.FromCtx(ctx).Warn("staging path unexpectedly disappeared")
			return &csi.NodeUnstageVolumeResponse{}, nil
		}
		return nil, fmt.Errorf("check remaining files in staging path: %w", err)
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

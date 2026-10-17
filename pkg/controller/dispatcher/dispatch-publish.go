package dispatcher

import (
	"context"
	"crypto/ecdh"
	"crypto/hpke"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/controller/blockshare"
	"meoe.io/poly-csi/pkg/node/nodeinfo"
	"meoe.io/poly-csi/pkg/noderegistry/noderegclient"
)

func (d *Dispatcher) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	// logger := logs.FromCtx(ctx)
	conf := FromCtx(ctx)

	mh, err := d.getExistingVolumeHandle(ctx, req.VolumeId, conf)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	if mh == nil {
		return nil, fmt.Errorf("volume not found")
	}

	nodeReg := noderegclient.New(d.conf.NodeInfoDb.Address)
	rawNodeInfo, err := nodeReg.Get(ctx, req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("node_info: failed to get: %w", err)
	}

	ni, err := nodeinfo.ParseNodeInfo(req.NodeId, rawNodeInfo)
	if err != nil {
		return nil, fmt.Errorf("node_info: parse: %w", err)
	}
	err = ni.Validate()
	if err != nil {
		return nil, fmt.Errorf("node_info: validation failed: %w", err)
	}

	// if mh.vi.VolumeContext.PresetName != "" {
	// 	for v := range strings.SplitSeq(mh.vi.VolumeContext.PresetName, ",") {
	// 		if !slices.Contains(ni.Presets, v) {
	// 			return nil, fmt.Errorf("node does not have preset '%s' required by volume (available %v)", v, ni.Presets)
	// 		}
	// 	}
	// }

	// conInfo, ok := ni.Connectors[mh.vi.ConnectorId]
	// if !ok {
	// 	return nil, fmt.Errorf("node info does not list connector %s", mh.vi.ConnectorId)
	// }

	kem := hpke.DHKEM(ecdh.X25519())
	nodeKey, err := kem.NewPublicKey(ni.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("load node public key: %w", err)
	}

	pc, err := mh.publish(ctx, blockshare.PublishRequest{
		NodeName: req.NodeId,
		// ConInfo:  conInfo.Info,
	})
	if err != nil {
		return nil, fmt.Errorf("shareHandle.Publish to %s: %w", req.NodeId, err)
	}
	// validate := validator.New(validator.WithRequiredStructEnabled())
	// err = validate.Struct(pc.Protocol)
	// if err != nil {
	// 	err = protocols.FormatValidationError(err)
	// 	return nil, fmt.Errorf("validate share protocol data: %w", err)
	// }

	result := map[string]string{}
	err = pc.Serialize(result, nodeKey)
	if err != nil {
		return nil, fmt.Errorf("serialize publish context: %w", err)
	}

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: result,
	}, nil
}

func (d *Dispatcher) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	conf := FromCtx(ctx)

	mh, err := d.getExistingVolumeHandle(ctx, req.VolumeId, conf)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	if mh == nil {
		return nil, fmt.Errorf("volume not found")
	}

	if req.NodeId == "" {
		err = mh.unpublishAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("unpublishAll: %w", err)
		}
	} else {
		err = mh.unpublish(ctx, req.NodeId)
		if err != nil {
			return nil, fmt.Errorf("unpublish: %w", err)
		}
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

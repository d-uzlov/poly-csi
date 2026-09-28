package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/controller/shares/sharefactory"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors/protocols"
	"meoe.io/poly-csi/pkg/node/nodeinfo"
	"meoe.io/poly-csi/pkg/noderegistry/nodremotereg"
	"meoe.io/poly-csi/pkg/utils/encrypt"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

func (d *Dispatcher) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	logger := logs.FromCtx(ctx)

	mh, err := d.getExistingVolumeHandle(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	shrInstance, err := sharefactory.ShareFromType(mh.srv.trInstance, mh.vi.ShareType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, fmt.Errorf("find share config: %w", err)
	}

	var addressOptions []AddressOption
	for _, id := range mh.vi.AddressIds {
		found := false
		for _, v := range mh.srv.Conf.AddressList {
			if v.StableId == id {
				addressOptions = append(addressOptions, v)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("address 'id=%s' is not found", id)
		}
	}

	nodeReg := nodremotereg.NewNodeRemoteRegistry(d.conf.NodeInfoDb.Address)
	rawNodeInfo, err := nodeReg.Get(req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("node_info: failed to get: %w", err)
	}
	if rawNodeInfo == nil {
		return nil, fmt.Errorf("node_info: node id not found")
	}

	ni, err := nodeinfo.ParseNodeInfo(rawNodeInfo.Data)
	if err != nil {
		return nil, fmt.Errorf("node_info: parse: %w", err)
	}
	err = ni.Validate()
	if err != nil {
		return nil, fmt.Errorf("node_info: validation failed: %w", err)
	}

	if mh.vi.VolumeContext.PresetName != "" {
		for v := range strings.SplitSeq(mh.vi.VolumeContext.PresetName, ",") {
			if !slices.Contains(ni.Presets, v) {
				return nil, fmt.Errorf("node does not have preset '%s' required by volume (available %v)", v, ni.Presets)
			}
		}
	}

	mountInfo, ok := ni.Connectors[mh.vi.ConnectorId]
	if !ok {
		return nil, fmt.Errorf("node info does not list connector %s", mh.vi.ConnectorId)
	}

	shareHandle, err := shrInstance.ParseShare(mh.vi.ShareData)
	if err != nil {
		return nil, fmt.Errorf("parse share data: %w", err)
	}
	if shareHandle == nil {
		return nil, fmt.Errorf("parse share data: nil result")
	}
	err = shareHandle.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate share: %w", err)
	}

	pc, err := shareHandle.Publish(ctx, mountInfo)
	if err != nil {
		return nil, fmt.Errorf("shareHandle.Publish to %s/%s: %w", req.NodeId, mh.vi.ConnectorId, err)
	}
	logger.Info("share.Publish: success")
	if pc == nil {
		return &csi.ControllerPublishVolumeResponse{}, nil
	}

	publishContext := map[string]string{}
	err = InjectAddressOptions(addressOptions, publishContext)
	if err != nil {
		return nil, fmt.Errorf("inject address options: %w", err)
	}

	if pc.Context == nil {
		return nil, fmt.Errorf("share did not set publish context")
	}

	public, secret, err := protocols.SplitSecret(pc.Context)

	_, err = serialmaps.SerializeInto(public, publishContext, "")
	if err != nil {
		return nil, fmt.Errorf("inject public data: %w", err)
	}

	logger.Info("creating encrypted publish secret")
	err = encrypt.InjectSecretIntoMap(publishContext, ni.PublicKey, secret)
	if err != nil {
		return nil, fmt.Errorf("inject secret: %w", err)
	}

	if pc.LocalData != nil {
		serialized, err := json.Marshal(pc.LocalData)
		if err != nil {
			return nil, fmt.Errorf("serialize local data: %w", err)
		}
		err = mh.wrapper.SetPublishCtx(ctx, req.NodeId, string(serialized))
		if err != nil {
			return nil, fmt.Errorf("publish_context: save to db: %w", err)
		}
	}

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: publishContext,
	}, nil
}

func (d *Dispatcher) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	mh, err := d.getExistingVolumeHandle(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	return d.unpublishVolume(ctx, mh, req.NodeId)
}

func (d *Dispatcher) unpublishVolume(ctx context.Context, mh *volumeManagementHandle, nodeId string) (*csi.ControllerUnpublishVolumeResponse, error) {
	logger := logs.FromCtx(ctx)

	shrInstance, err := sharefactory.ShareFromType(mh.srv.trInstance, mh.vi.ShareType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, fmt.Errorf("find share config: %w", err)
	}

	shareHandle, err := shrInstance.ParseShare(mh.vi.ShareData)
	if err != nil {
		return nil, fmt.Errorf("parse share data: %w", err)
	}
	if shareHandle == nil {
		return nil, fmt.Errorf("parse share data: nil result")
	}
	err = shareHandle.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate share: %w", err)
	}

	if nodeId != "" {
		pds, err := mh.wrapper.GetPublishData(ctx, nodeId)
		if err != nil {
			return nil, fmt.Errorf("publish data: get from db: %w", err)
		}

		// nothing to do if publish data is empty
		if pds == "" {
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}

		err = shareHandle.Unpublish(ctx, pds)
		if err != nil {
			return nil, fmt.Errorf("shareHandle.Unpublish: %w", err)
		}
		logger.Info("share.Unpublish: success")

		err = mh.wrapper.DeletePublishData(ctx, nodeId)
		if err != nil {
			return nil, fmt.Errorf("db: delete publish context: %w", err)
		}
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	caps := shrInstance.GetCapabilities()
	if caps.UnpublishAll {
		err = shareHandle.UnpublishAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("shareHandle.UnpublishAll: %w", err)
		}
		logger.Info("share.UnpublishAll: success")
		err = mh.wrapper.ClearPublishData(ctx)
		if err != nil {
			return nil, fmt.Errorf("ClearPublishData: %w", err)
		}
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	fullPublishData, err := mh.wrapper.GetFullPublishData(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetFullPublishCtxList: %w", err)
	}

	for _, v := range fullPublishData {
		if v == "" {
			// nothing to do if publish data is empty
			continue
		}

		err = shareHandle.Unpublish(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("shareHandle.Unpublish: %w", err)
		}
	}

	logger.Info("share.Unpublish: success for all entries")

	err = mh.wrapper.ClearPublishData(ctx)
	if err != nil {
		return nil, fmt.Errorf("ClearPublishData: %w", err)
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

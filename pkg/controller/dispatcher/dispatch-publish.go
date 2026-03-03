package dispatcher

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/controller/shares/sharefactory"
	"meoe.io/poly-csi/pkg/node/nodeinfo"
	"meoe.io/poly-csi/pkg/noderegistry/nodremotereg"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

func (d *Dispatcher) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	logger := slogctx.FromCtx(ctx)

	mh, err := d.getVolumeInfo(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	shrInstance, err := sharefactory.ShareFromType(mh.srv.trInstance, mh.vi.ShareType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
	if err != nil {
		return nil, fmt.Errorf("find share config: %w", err)
	}

	var addressOptions []AddressOption
	for _, id := range mh.vi.AddressIds {
		found := false
		for _, v := range mh.srv.conf.AddressList {
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

	nodeReg := nodremotereg.NewNodeRemoteRegistry(d.conf.nodeInfoDb.Address)
	rawNodeInfo, err := nodeReg.Get(req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("node_info: failed to get: %w", err)
	}
	if rawNodeInfo == nil {
		return nil, fmt.Errorf("node_info: node id not found")
	}

	ni, err := nodeinfo.ParseNodeInfo(rawNodeInfo.Data, d.conf.encrypt.key)
	if err != nil {
		return nil, fmt.Errorf("node_info: parse: %w", err)
	}
	err = ni.Validate()
	if err != nil {
		return nil, fmt.Errorf("node_info: validation failed: %w", err)
	}

	mountInfo, ok := ni.Connectors[mh.vi.ConnectorId]
	if !ok {
		return nil, fmt.Errorf("node info does not list connector %s", mh.vi.ConnectorId)
	}

	pc, err := shrInstance.Publish(ctx, mh.vi.ShareData, mountInfo)
	if err != nil {
		return nil, fmt.Errorf("shareInstance.Publish to %s/%s: %w", req.NodeId, mh.vi.ConnectorId, err)
	}
	logger.Info("share.Publish: success")
	if pc == nil {
		return &csi.ControllerPublishVolumeResponse{}, nil
	}

	if pc.Public == nil {
		pc.Public = map[string]string{}
	}
	err = InjectAddressOptions(addressOptions, pc.Public)
	if err != nil {
		return nil, fmt.Errorf("inject address options: %w", err)
	}

	if pc.Secret != nil {
		logger.Info("creating encrypted publish context")

		aesSecret := make([]byte, 32)
		_, err = rand.Read(aesSecret)
		if err != nil {
			return nil, fmt.Errorf("generate random bytes for aes: %w", err)
		}
		encrypt.InjectAesSecret(pc.Public, *ni.PublicKey, aesSecret)
		secretJson, err := json.Marshal(pc.Secret)
		if err != nil {
			return nil, fmt.Errorf("publish_context: marshal secret: %w", err)
		}
		secretJsonEnc, err := encrypt.EncryptAes(aesSecret, string(secretJson))
		if err != nil {
			return nil, fmt.Errorf("publish_context: encrypt secret: %w", err)
		}
		pc.Public["_secret"] = base64.StdEncoding.EncodeToString(secretJsonEnc)
	}

	if pc.Data != nil {
		serialized, err := pc.Data.Serialize()
		if err != nil {
			return nil, fmt.Errorf("serialize publish data: %w", err)
		}
		err = mh.wrapper.SetPublishCtx(ctx, req.NodeId, serialized)
		if err != nil {
			return nil, fmt.Errorf("publish_context: save to db: %w", err)
		}
	}

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: pc.Public,
	}, nil
}

func (d *Dispatcher) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	logger := slogctx.FromCtx(ctx)

	mh, err := d.getVolumeInfo(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	shrInstance, err := sharefactory.ShareFromType(mh.srv.trInstance, mh.vi.ShareType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
	if err != nil {
		return nil, fmt.Errorf("find share config: %w", err)
	}

	if req.NodeId != "" {
		pds, err := mh.wrapper.GetPublishData(ctx, req.NodeId)
		if err != nil {
			return nil, fmt.Errorf("publish data: get from db: %w", err)
		}

		// nothing to do if publish data is empty
		if pds != "" {
			err = shrInstance.Unpublish(ctx, mh.vi.ShareData, pds)
			if err != nil {
				return nil, fmt.Errorf("shrInstance.Unpublish: %w", err)
			}
			logger.Info("share.Unpublish: success")

			err = mh.wrapper.DeletePublishData(ctx, req.NodeId)
			if err != nil {
				return nil, fmt.Errorf("db: delete publish context: %w", err)
			}
		}
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	caps := shrInstance.GetCapabilities()
	if caps.UnpublishAll {
		err = shrInstance.UnpublishAll(ctx, mh.vi.ShareData)
		if err != nil {
			return nil, fmt.Errorf("shrInstance.UnpublishAll: %w", err)
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

		err = shrInstance.Unpublish(ctx, mh.vi.ShareData, v)
		if err != nil {
			return nil, fmt.Errorf("shrInstance.Unpublish: %w", err)
		}
	}

	logger.Info("share.Unpublish: success for all entries")

	err = mh.wrapper.ClearPublishData(ctx)
	if err != nil {
		return nil, fmt.Errorf("ClearPublishData: %w", err)
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

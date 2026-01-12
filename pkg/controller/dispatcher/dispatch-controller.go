package dispatcher

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/shares/sharefactory"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/storagefactory"
	"meoe.io/poly-csi/pkg/controller/voldbwrapper"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/nodeinfo"
	"meoe.io/poly-csi/pkg/noderegistry/nodremotereg"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

type IdFields struct {
	RequestName string
	Uuid        string
	K8sNs       string
	K8sPvc      string
}

func generateId(tmplName string, tmpl string, requestName string, id string, staticParams map[string]string) (string, error) {
	t, err := template.New(tmplName).Parse(tmpl)
	if err != nil {
		return "", err
	}

	buf := bytes.Buffer{}
	err = t.Execute(&buf, IdFields{
		RequestName: requestName,
		Uuid:        id,
		K8sNs:       staticParams["csi.storage.k8s.io/pvc/namespace"],
		K8sPvc:      staticParams["csi.storage.k8s.io/pvc/name"],
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ==== create-delete ====

func (d *Dispatcher) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (resp *csi.CreateVolumeResponse, finalErr error) {
	logger := slogctx.FromCtx(ctx)

	dispatchParams, err := parseDispatchParams(nil, req.Parameters, req.MutableParameters)
	if err != nil {
		return nil, fmt.Errorf("parseDispatchParams: %w", err)
	}

	srv, err := d.FindServerByName(dispatchParams.serverName)
	if err != nil {
		return nil, fmt.Errorf("server[name=%s]: not found", dispatchParams.serverName)
	}
	srv.mutex.RLock()
	defer srv.mutex.RUnlock()

	err = srv.Check()
	if err != nil {
		return nil, fmt.Errorf("server[name=%s]: %w", dispatchParams.serverName, err)
	}

	ctx = transport.MakeContext(ctx, srv.trInstance)

	str, err := storagefactory.StorageFromType(dispatchParams.storageType, srv.conf.Commands, srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	shr, err := sharefactory.ShareFromType(dispatchParams.shareType, srv.conf.Commands, srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	strCaps := str.GetCapabilities()
	shrCaps := shr.GetCapabilities()

	if shrCaps.LocalVolume != nil {
		if !slices.Contains(shrCaps.LocalVolume, strCaps.LocalVolume) {
			return nil, fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", dispatchParams.shareType, shrCaps.LocalVolume, dispatchParams.storageType, strCaps.LocalVolume)
		}
	}

	if shrCaps.OpaqueVolume != nil {
		if !slices.Contains(shrCaps.OpaqueVolume, strCaps.OpaqueVolume) {
			return nil, fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", dispatchParams.shareType, shrCaps.OpaqueVolume, dispatchParams.storageType, strCaps.OpaqueVolume)
		}
	}

	var addressOptions []AddressOption
	for name := range strings.SplitSeq(dispatchParams.addressNames, ",") {
		found := false
		for _, v := range srv.conf.AddressList {
			if v.Name == name {
				addressOptions = append(addressOptions, v)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("address '%s' is not found", name)
		}
	}

	volumeId := createVolumeId(srv.conf.StableId, req.Name)

	existingDbEntry, err := srv.dbInstance.GetEntry(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("check existing volume: %w", err)
	}
	if existingDbEntry != nil {
		// wrapped := voldbwrapper.New(existingDbEntry)
		// TODO is this wrong? Should we even have this check?
		vgr, err := d.ControllerGetVolume(ctx, &csi.ControllerGetVolumeRequest{VolumeId: volumeId})
		if err != nil {
			return nil, fmt.Errorf("volume '%s' exists in the database but we failed to get it from underlying storage: %w", volumeId, err)
		}
		return &csi.CreateVolumeResponse{
			Volume: vgr.Volume,
		}, nil
	}

	uuidString := uuid.NewString()

	success := false

	storageId := req.Name
	if dispatchParams.storageIdTemplate != "" {
		id, err := generateId("storageIdTemplate", dispatchParams.storageIdTemplate, req.Name, uuidString, req.Parameters)
		if err != nil {
			return nil, fmt.Errorf("generate storage id: %w", err)
		}
		storageId = id
	}

	size := req.CapacityRange.RequiredBytes

	storageCreateRequest := storage.CreateRequest{
		Id:            storageId,
		Size:          size,
		StaticParams:  req.Parameters,
		MutableParams: req.MutableParameters,
	}

	var vol storage.Volume
	if req.VolumeContentSource != nil {
		if snap := req.VolumeContentSource.GetSnapshot(); snap != nil {
			return nil, fmt.Errorf("snapshot source is not implemented")
		} else if src := req.VolumeContentSource.GetVolume(); src != nil {
			vol, err = d.optimizedClone(ctx, src.VolumeId, srv, storageCreateRequest, dispatchParams)
			if err != nil {
				return nil, fmt.Errorf("optimizedClone volume_id=%s: %w", src.VolumeId, err)
			}
			if vol == nil {
				return nil, fmt.Errorf("clone did not return storage context")
			}
		} else {
			return nil, fmt.Errorf("unknown volume content source type: %#v", req.VolumeContentSource.Type)
		}
	} else {
		vol, err = str.CreateVolume(ctx, storageCreateRequest)
		if err != nil {
			return nil, fmt.Errorf("storage[%s]:CreateVolume: %w", dispatchParams.storageType, err)
		}
		logger.Info("storage.CreateVolume: success")
	}

	if vol == nil {
		return nil, fmt.Errorf("volume create did not set storage context")
	}

	defer func() {
		if success {
			return
		}
		logger.Info("deleting storage volume on failure")
		slogctx.FromCtx(ctx).Debug("running storage cleanup")
		err = vol.Delete(ctx)
		if err != nil {
			slogctx.FromCtx(ctx).Error("failed to run storage cleanup", "error", err)
			return
		}
	}()

	// when using local volumes, we can check common stuff here, to avoid repeating it in each share
	if strCaps.LocalVolume != "" {
		localPath, err := vol.GetLocalPath(ctx)
		if err != nil {
			return nil, fmt.Errorf("get local file: %w", err)
		}
		err = checkFile(ctx, strCaps.LocalVolume, localPath)
		if err != nil {
			return nil, fmt.Errorf("check local file: %w", err)
		}
	}

	shareId := req.Name
	if dispatchParams.shareIdTemplate != "" {
		id, err := generateId("shareIdTemplate", dispatchParams.shareIdTemplate, req.Name, uuidString, req.Parameters)
		if err != nil {
			return nil, fmt.Errorf("generate share id: %w", err)
		}
		shareId = id
	}

	shareData, err := shr.CreateShare(ctx, shares.CreateShareRequest{
		ShareId:       shareId,
		StaticParams:  req.Parameters,
		MutableParams: req.MutableParameters,
		LocalVolume:   strCaps.LocalVolume,
		OpaqueVolume:  strCaps.OpaqueVolume,
		Volume:        vol,
	})
	if err != nil {
		return nil, fmt.Errorf("create share: %w", err)
	}
	logger.Info("share.CreateShare: success")
	defer func() {
		if success {
			return
		}
		logger.Info("deleting share on failure")
		slogctx.FromCtx(ctx).Debug("running share cleanup")
		err = shr.DeleteShare(ctx, shareData)
		if err != nil {
			slogctx.FromCtx(ctx).Error("failed to run share cleanup", "error", err)
			return
		}
	}()

	volumeContext := map[string]string{}
	volumeContext[mounts.MountIdKey] = dispatchParams.mounterId

	var addressIds []string
	for _, v := range addressOptions {
		addressIds = append(addressIds, v.StableId)
	}

	serializedVolume, err := vol.Serialize()
	if err != nil {
		return nil, fmt.Errorf("serialize storage volume: %w", err)
	}

	dbEntry, err := srv.dbInstance.CreateEntry(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("create db entry: %w", err)
	}
	wrapped := voldbwrapper.New(dbEntry)
	err = wrapped.SetVolumeInfo(ctx, voldbwrapper.VolumeInfo{
		MounterId:     dispatchParams.mounterId,
		TransportId:   srv.conf.StableId,
		VolumeContext: volumeContext,
		AddressIds:    addressIds,
		StorageType:   dispatchParams.storageType,
		StorageData:   serializedVolume,
		ShareType:     dispatchParams.shareType,
		ShareData:     shareData,
	})
	if err != nil {
		return nil, fmt.Errorf("save volume info: %w", err)
	}

	success = true
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeId,
			CapacityBytes: 0,
			VolumeContext: volumeContext,
		},
	}, nil
}

func (d *Dispatcher) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	logger := slogctx.FromCtx(ctx)

	mh, err := d.getVolumeInfo(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	ctx = transport.MakeContext(ctx, mh.srv.trInstance)

	strInstance, err := storagefactory.StorageFromType(mh.vi.StorageType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	shrInstance, err := sharefactory.ShareFromType(mh.vi.ShareType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	err = shrInstance.DeleteShare(ctx, mh.vi.ShareData)
	if err != nil {
		return nil, fmt.Errorf("delete share: %w", err)
	}
	logger.Info("share.DeleteShare: success")

	vol, err := strInstance.ParseVolume(mh.vi.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	err = vol.Delete(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete storage: %w", err)
	}
	logger.Info("storage.DeleteVolume: success")

	err = mh.wrapper.Delete(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete db entry: %w", err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

// ==== publish ====

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

	ctx = transport.MakeContext(ctx, mh.srv.trInstance)

	shrInstance, err := sharefactory.ShareFromType(mh.vi.ShareType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
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

	var mountInfo *mounts.MounterNodeInfo
	for _, v := range ni.Mounters {
		if v.Id == mh.vi.MounterId {
			mountInfo = &v
			break
		}
	}
	if mountInfo == nil {
		return nil, fmt.Errorf("node info does not list mounter %s", mh.vi.MounterId)
	}

	pc, err := shrInstance.Publish(ctx, mh.vi.ShareData, *mountInfo)
	if err != nil {
		return nil, fmt.Errorf("shareInstance.Publish: %w", err)
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

	if pc.Local != nil {
		err = mh.wrapper.CreatePublishCtx(ctx, req.NodeId, pc.Local)
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

	ctx = transport.MakeContext(ctx, mh.srv.trInstance)

	shrInstance, err := sharefactory.ShareFromType(mh.vi.ShareType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
	if err != nil {
		return nil, fmt.Errorf("find share config: %w", err)
	}

	if req.NodeId == "" {
		err = shrInstance.UnpublishAll(ctx, mh.vi.ShareData)
		if err != nil {
			return nil, fmt.Errorf("shrInstance.UnpublishAll: %w", err)
		}
		logger.Info("share.UnpublishAll: success")
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	pcLocal, err := mh.wrapper.GetPublishCtx(ctx, req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("publish_context: get from db: %w", err)
	}
	if pcLocal == nil {
		// nothing to do if publish context is not found
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	err = shrInstance.Unpublish(ctx, mh.vi.ShareData, pcLocal)
	if err != nil {
		return nil, fmt.Errorf("shrInstance.Unpublish: %w", err)
	}
	logger.Info("share.Unpublish: success")

	err = mh.wrapper.DeletePublishCtx(ctx, req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("db: delete publish context: %w", err)
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// ==== misc ====

func (d *Dispatcher) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	dispatchParams, err := parseDispatchParams(nil, req.Parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("parseDispatchParams: %w", err)
	}

	srv, err := d.FindServerByName(dispatchParams.serverName)
	if err != nil {
		return nil, fmt.Errorf("server[name=%s]: not found", dispatchParams.serverName)
	}
	srv.mutex.RLock()
	defer srv.mutex.RUnlock()

	err = srv.Check()
	if err != nil {
		return nil, fmt.Errorf("server[name=%s]: %w", dispatchParams.serverName, err)
	}

	ctx = transport.MakeContext(ctx, srv.trInstance)

	strInstance, err := storagefactory.StorageFromType(dispatchParams.storageType, srv.conf.Commands, srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	available, err := strInstance.GetAvailableCapacity(ctx, req.Parameters)
	if err != nil {
		return nil, fmt.Errorf("server[name=%s]: storage[%s]: get volume size: %w", dispatchParams.serverName, dispatchParams.storageType, err)
	}

	return &csi.GetCapacityResponse{
		AvailableCapacity: available,
	}, nil
}

// func (d *Dispatcher) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
// 	connectionName, ok := req.Parameters["dispatch.name"]
// 	if !ok {
// 		return nil, fmt.Errorf("parameters: dispatch.name is missing")
// 	}

// 	conn, ok := d.NameMap[connectionName]
// 	if !ok {
// 		return nil, fmt.Errorf("connection is not found (by name): %s", connectionName)
// 	}

// 	return conn.volumeManager.GetCapacity(volumemanager.ContextWithTransport(ctx, conn.transport), req)
// }

func (d *Dispatcher) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	mh, err := d.getVolumeInfo(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	ctx = transport.MakeContext(ctx, mh.srv.trInstance)

	strInstance, err := storagefactory.StorageFromType(mh.vi.StorageType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	vol, err := strInstance.ParseVolume(mh.vi.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	volumeSize, err := vol.GetSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("strInstance.GetVolumeSize: %w", err)
	}

	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      req.VolumeId,
			CapacityBytes: volumeSize,
			VolumeContext: mh.vi.VolumeContext,
		},
		Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: false,
				Message:  "ok",
			},
		},
	}, nil
}

// func (d *Dispatcher) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
// 	d.mainMutex.RLock()
// 	defer d.mainMutex.RUnlock()

// 	conId, originalVolId, err := parseVolumeId(req.VolumeId)
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid volume id: %w", err)
// 	}

// 	con, release, err := d.GetConnectionById(conId)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer release()

// 	req.VolumeId = originalVolId
// 	return con.volumeManager.ControllerModifyVolume(transport.MakeContext(ctx, con.transport), req)
// }

func (d *Dispatcher) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	logger := slogctx.FromCtx(ctx)

	mh, err := d.getVolumeInfo(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	ctx = transport.MakeContext(ctx, mh.srv.trInstance)

	strInstance, err := storagefactory.StorageFromType(mh.vi.StorageType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	vol, err := strInstance.ParseVolume(mh.vi.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	newSize := req.CapacityRange.RequiredBytes

	volumeSize, err := vol.Expand(ctx, newSize)
	if err != nil {
		return nil, fmt.Errorf("storage: expand volume: %w", err)
	}
	logger.Info("storage.Expand: success")

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         volumeSize,
		NodeExpansionRequired: true,
	}, nil
}

// ==== snapshots ====

func (d *Dispatcher) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	mh, err := d.getVolumeInfo(ctx, req.SourceVolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	// TODO calling method on nil pointer??
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	snapshotId := createSnapshotId(mh.srv.conf.StableId, mh.volumeName, req.Name)

	ctx = transport.MakeContext(ctx, mh.srv.trInstance)

	strInstance, err := storagefactory.StorageFromType(mh.vi.StorageType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	vol, err := strInstance.ParseVolume(mh.vi.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	volSize, err := vol.GetSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("get volume size: %w", err)
	}

	snap, err := vol.MakeNativeSnapshot(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("create snapshot in storage: %w", err)
	}

	snapData, err := snap.Serialize()
	if err != nil {
		return nil, fmt.Errorf("serialize snapshot: %w", err)
	}

	err = mh.wrapper.CreateSnapshot(ctx, req.Name, voldbwrapper.SnapshotInfo{
		SnapshotType:   "native",
		StorageType:    mh.vi.StorageType,
		SerializedData: snapData,
	})
	if err != nil {
		return nil, fmt.Errorf("save snapshot data: %w", err)
	}

	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshotId,
			SourceVolumeId: req.SourceVolumeId,
			CreationTime:   nil, // TODO
			SizeBytes:      volSize,
			ReadyToUse:     true,
		},
	}, nil
}

func (d *Dispatcher) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	mh, err := d.getSnapshotHandle(ctx, req.SnapshotId)
	if err != nil {
		return nil, fmt.Errorf("get snapshot handle: %w", err)
	}
	defer mh.release()
	if mh.si == nil {
		return nil, fmt.Errorf("snapshot not found")
	}

	ctx = transport.MakeContext(ctx, mh.srv.trInstance)

	strInstance, err := storagefactory.StorageFromType(mh.si.StorageType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	// vol, err := strInstance.ParseVolume(mh.vi.StorageData)
	// if err != nil {
	// 	return nil, fmt.Errorf("parse storage data: %w", err)
	// }

	snap, err := strInstance.ParseNativeSnapshot(mh.si.SerializedData)
	if err != nil {
		return nil, fmt.Errorf("parse snapshot data: %w", err)
	}

	err = snap.Delete(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete underlying snapshot: %w", err)
	}

	err = mh.wrapper.DeleteSnapshot(ctx, mh.snapshotName)
	if err != nil {
		return nil, fmt.Errorf("delete snapshot data from database: %w", err)
	}

	return &csi.DeleteSnapshotResponse{}, nil
}

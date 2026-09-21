package dispatcher

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/shares/sharefactory"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/storagefactory"
	"meoe.io/poly-csi/pkg/controller/uuidctx"
	"meoe.io/poly-csi/pkg/controller/voldbwrapper"
	"meoe.io/poly-csi/pkg/controller/volumecontext"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
	"meoe.io/poly-csi/pkg/utils/volumeinit"
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

func (d *Dispatcher) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (resp *csi.CreateVolumeResponse, finalErr error) {
	logger := logs.FromCtx(ctx)

	dispatchParams, err := parseDispatchParams(nil, req.Parameters, req.MutableParameters)
	if err != nil {
		return nil, fmt.Errorf("parseDispatchParams: %w", err)
	}

	srv, err := d.FindServerByName(dispatchParams.ServerName)
	if err != nil {
		return nil, fmt.Errorf("server[name=%s]: not found", dispatchParams.ServerName)
	}
	srv.mutex.RLock()
	defer srv.mutex.RUnlock()

	err = srv.Check()
	if err != nil {
		return nil, fmt.Errorf("server[name=%s]: %w", dispatchParams.ServerName, err)
	}

	str, err := storagefactory.StorageFromType(srv.trInstance, dispatchParams.StorageType, srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	shr, err := sharefactory.ShareFromType(srv.trInstance, dispatchParams.ShareType, srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	var initConfig volumeinit.InitConfig
	err = serialmaps.ParseRemove(req.Parameters, volumeinit.PreferredPrefix, true, &initConfig)
	if err != nil {
		return nil, fmt.Errorf("parse init config: %w", err)
	}
	err = initConfig.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate init config: %w", err)
	}
	for k := range req.Parameters {
		if strings.HasPrefix(k, volumeinit.PreferredPrefix) {
			return nil, fmt.Errorf("unknown key '%s'", k)
		}
	}

	strCaps := str.GetCapabilities()
	shrCaps := shr.GetCapabilities()

	if shrCaps.LocalVolume != nil {
		if !slices.Contains(shrCaps.LocalVolume, strCaps.LocalPathType) {
			return nil, fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", dispatchParams.ShareType, shrCaps.LocalVolume, dispatchParams.StorageType, strCaps.LocalPathType)
		}
	}

	if shrCaps.OpaqueVolume != nil {
		if !slices.Contains(shrCaps.OpaqueVolume, strCaps.OpaqueVolumeType) {
			return nil, fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", dispatchParams.ShareType, shrCaps.OpaqueVolume, dispatchParams.StorageType, strCaps.OpaqueVolumeType)
		}
	}

	var addressOptions []AddressOption
	for name := range strings.SplitSeq(dispatchParams.AddressNames, ",") {
		found := false
		for _, v := range srv.Conf.AddressList {
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

	if strings.Contains(req.Name, "/") {
		return nil, fmt.Errorf("volume name must not contain '/' symbol")
	}
	volumeId := createVolumeId(srv.Conf.StableId, req.Name)

	existingDbEntry, err := srv.dbInstance.FindEntry(ctx, req.Name)
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

	callUuid := uuidctx.FromCtx(ctx)

	success := false

	storageId := req.Name
	if dispatchParams.StorageIdTemplate != "" {
		id, err := generateId("storageIdTemplate", dispatchParams.StorageIdTemplate, req.Name, callUuid, req.Parameters)
		if err != nil {
			return nil, fmt.Errorf("generate storage id: %w", err)
		}
		storageId = id
	}

	storageCreateRequest := storage.CreateRequest{
		Id:            storageId,
		StaticParams:  req.Parameters,
		MutableParams: req.MutableParameters,
	}

	vol, err := str.CreateVolume(storageCreateRequest)
	if err != nil {
		return nil, fmt.Errorf("storage[%s]:CreateVolume: %w", dispatchParams.StorageType, err)
	}
	if vol == nil {
		return nil, fmt.Errorf("storage[%s]:CreateVolume: nil result", dispatchParams.StorageType)
	}
	defer func() {
		if success {
			return
		}
		logger.Info("deleting storage volume on failure")
		logs.FromCtx(ctx).Info("running storage cleanup")
		err = vol.DeleteState(ctx)
		if err != nil {
			logs.FromCtx(ctx).Error("failed to run storage cleanup", "error", err.Error())
			return
		}
	}()

	volumeState, err := vol.ReadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if volumeState == nil {
		if req.VolumeContentSource != nil {
			var sourceSnapshot storage.Snapshot
			var sourceServer *parsedServer
			var sourceCaps *storage.Capabilities

			tmpSnapshotId := "snap-for-" + req.Name

			if src := req.VolumeContentSource.GetSnapshot(); src != nil {
				sh, err := d.getSnapshotHandle(ctx, src.SnapshotId)
				if err != nil {
					return nil, fmt.Errorf("getSnapshotHandle: %w", err)
				}
				defer sh.release()
				if sh.si == nil {
					return nil, fmt.Errorf("source snapshot is not found")
				}

				if sh.si.SnapshotType != "native" {
					return nil, fmt.Errorf("clone from non-native snapshot is not supported")
				}

				sourceServer = sh.srv

				sourceStr, err := storagefactory.StorageFromType(sh.srv.trInstance, sh.si.StorageType, sh.srv.Conf.GlobalParams)
				if err != nil {
					return nil, fmt.Errorf("get source storage: %w", err)
				}

				caps := sourceStr.GetCapabilities()
				sourceCaps = &caps

				snap, err := sourceStr.ParseSnapshot(sh.si.SerializedData)
				if err != nil {
					return nil, fmt.Errorf("parse source snapshot data: %w", err)
				}
				sourceSnapshot = snap
			} else if src := req.VolumeContentSource.GetVolume(); src != nil {
				sh, err := d.getVolumeInfo(ctx, src.VolumeId)
				if err != nil {
					return nil, fmt.Errorf("get volume info: %w", err)
				}
				defer sh.release()
				if sh.vi == nil {
					return nil, fmt.Errorf("source volume is not found")
				}

				sourceServer = sh.srv

				sourceStr, err := storagefactory.StorageFromType(sh.srv.trInstance, sh.vi.StorageType, sh.srv.Conf.GlobalParams)
				if err != nil {
					return nil, fmt.Errorf("get source storage: %w", err)
				}

				caps := sourceStr.GetCapabilities()
				sourceCaps = &caps

				sourceVol, err := sourceStr.ParseVolume(sh.vi.StorageData)
				if err != nil {
					return nil, fmt.Errorf("parse source storage data: %w", err)
				}

				sourceCaps := sourceStr.GetCapabilities()
				if sourceCaps.NativeSnapshot {
					sourceSnapshot, err = sourceVol.MakeSnapshot(ctx, tmpSnapshotId)
					if err != nil {
						return nil, fmt.Errorf("create source snapshot: %w", err)
					}
					// TODO cleanup should use a different context, with extended timeout
					defer sourceSnapshot.Delete(ctx)
				} else {
					return nil, fmt.Errorf("source volume does not support native snapshots")
				}
			} else {
				return nil, fmt.Errorf("unknown volume content source type: %#v", req.VolumeContentSource.Type)
			}

			if sourceCaps == nil {
				return nil, fmt.Errorf("unexpectedly missing source capabilities")
			}
			if sourceSnapshot == nil {
				return nil, fmt.Errorf("unexpectedly missing source snapshot data")
			}
			if sourceServer == nil {
				return nil, fmt.Errorf("unexpectedly missing source server")
			}

			var sendFunc sendFuncType
			var receiveFunc receiveFuncType

			targetCaps := str.GetCapabilities()
			if sourceCaps.LocalSendType != "" && targetCaps.LocalSendType != "" &&
				sourceCaps.LocalSendType == targetCaps.LocalSendType {
				// Both source and target has send-receive support, and use the same format
				sendFunc = func(ctx context.Context, targetFile string, timeout time.Duration) error {
					return sourceSnapshot.Send(ctx, targetFile, timeout)
				}
				receiveFunc = func(ctx context.Context, sourceFile string, timeout time.Duration) error {
					return vol.ReceiveSnapshot(ctx, sourceFile, timeout)
				}
			} else if sourceCaps.LocalPathType != "" && targetCaps.LocalPathType != "" &&
				sourceCaps.LocalPathType == targetCaps.LocalPathType {
				// File-based volumes, can copy contents
				sourcePath, releaseSnapshot, err := sourceSnapshot.GetLocalPath(ctx)
				if err != nil {
					return nil, fmt.Errorf("get snapshot local path: %w", err)
				}
				defer releaseSnapshot()

				err = vol.CreateState(ctx, req.CapacityRange.RequiredBytes)
				if err != nil {
					return nil, fmt.Errorf("CreateState: %w", err)
				}
				volumeState, err = vol.ReadState(ctx)
				if err != nil {
					return nil, fmt.Errorf("storage[%s]:CreateVolume:ReadState: %w", dispatchParams.StorageType, err)
				}
				if volumeState == nil {
					return nil, fmt.Errorf("storage[%s]:CreateVolume:ReadState: nil result", dispatchParams.StorageType)
				}
				logger.Info("storage.CreateVolume: success")

				switch sourceCaps.LocalPathType {
				case storage.VolFolder:
					sendFunc = func(ctx context.Context, targetFile string, timeout time.Duration) error {
						return sendTar(ctx, sourceServer.trInstance, sourcePath, targetFile, timeout)
					}
					receiveFunc = func(ctx context.Context, sourceFile string, timeout time.Duration) error {
						err := receiveTar(ctx, srv.trInstance, sourceFile, volumeState.FsPath, timeout)
						if err != nil {
							return fmt.Errorf("receive tar: %w", err)
						}
						return nil
					}
				case storage.VolDevice:
					sendFunc = func(ctx context.Context, targetFile string, timeout time.Duration) error {
						return sendFile(ctx, sourceServer.trInstance, sourcePath, targetFile, timeout)
					}
					receiveFunc = func(ctx context.Context, sourceFile string, timeout time.Duration) error {
						err := receiveFile(ctx, srv.trInstance, sourceFile, volumeState.FsPath, timeout)
						if err != nil {
							return fmt.Errorf("receive device file: %w", err)
						}
						return nil
					}
				default:
					return nil, fmt.Errorf("unknown local volume type: %s", sourceCaps.LocalPathType)
				}
			} else {
				return nil, fmt.Errorf("source and target use incompatible send-receive mechanism")
			}

			// TODO better timeout
			timeout := time.Minute

			sendPipe, receivePipe, release, err := createClonePipes(ctx, tmpSnapshotId, sourceServer, srv, timeout)
			if release != nil {
				defer release()
			}
			if err != nil {
				return nil, fmt.Errorf("createClonePipes: %w", err)
			}

			sendCtx, sendCancel := context.WithTimeout(ctx, timeout)
			defer sendCancel()
			sendCh := make(chan error, 1)
			go func() {
				defer close(sendCh)
				sendCh <- sendFunc(sendCtx, sendPipe, timeout)
			}()

			receiveCtx, receiveCancel := context.WithTimeout(ctx, timeout)
			defer receiveCancel()

			receiveCh := make(chan error, 1)
			go func() {
				defer close(receiveCh)
				err := receiveFunc(receiveCtx, receivePipe, timeout)
				receiveCh <- err
			}()

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("wait send: %w", ctx.Err())
			case err = <-sendCh:
				if err != nil {
					return nil, fmt.Errorf("send: %w", err)
				}
				logger.Info("storage.Send: success")
				// send OK, now let's check receive
			}

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("wait receive: %w", ctx.Err())
			case err = <-receiveCh:
				if err != nil {
					return nil, fmt.Errorf("storage[%s]:receive: %w", dispatchParams.StorageType, err)
				}
				logger.Info("storage.Receive: success")
			}
		} else {
			err = vol.CreateState(ctx, req.CapacityRange.RequiredBytes)
			if err != nil {
				return nil, fmt.Errorf("CreateState: %w", err)
			}
			logger.Info("storage.CreateVolume: success")
		}
	}

	// read new volumeState
	volumeState, err = vol.ReadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if volumeState == nil {
		return nil, fmt.Errorf("ReadState: nil result")
	}
	if volumeState.Size < req.CapacityRange.RequiredBytes {
		err = vol.SetSize(ctx, req.CapacityRange.RequiredBytes)
		if err != nil {
			return nil, fmt.Errorf("SetSize: %w", err)
		}
		volumeState, err = vol.ReadState(ctx)
		if err != nil {
			return nil, fmt.Errorf("read state: %w", err)
		}
	}
	if strCaps.LocalPathType != "" {
		if volumeState.FsPath == "" {
			return nil, fmt.Errorf("read state: empty fs path")
		}

		// when using local volumes, we can check common stuff here, to avoid repeating it in each share
		err = checkFile(ctx, srv.trInstance, strCaps.LocalPathType, volumeState.FsPath)
		if err != nil {
			return nil, fmt.Errorf("check local file: %w", err)
		}

		init := volumeinit.NewController(srv.trInstance)
		switch strCaps.LocalPathType {
		case storage.VolDevice:
			logs.FromCtx(ctx).MicroOp("running volume init", "volume-type", strCaps.LocalPathType)
			_, err = init.RunBlock(ctx, initConfig, volumeState.FsPath)
			if err != nil {
				return nil, fmt.Errorf("init block: %w", err)
			}
			err = init.RunFsWithTmpMount(ctx, initConfig, volumeState.FsPath)
			if err != nil {
				return nil, fmt.Errorf("init fs: %w", err)
			}
		case storage.VolFolder:
			logs.FromCtx(ctx).MicroOp("running volume init", "volume-type", strCaps.LocalPathType)
			err = init.RunFs(ctx, initConfig, volumeState.FsPath)
			if err != nil {
				return nil, fmt.Errorf("init fs: %w", err)
			}
		default:
			logs.FromCtx(ctx).MicroOp("skipping volume init", "volume-type", strCaps.LocalPathType)
		}
	}

	shareId := req.Name
	if dispatchParams.ShareIdTemplate != "" {
		id, err := generateId("shareIdTemplate", dispatchParams.ShareIdTemplate, req.Name, callUuid, req.Parameters)
		if err != nil {
			return nil, fmt.Errorf("generate share id: %w", err)
		}
		shareId = id
	}

	shareHandle, err := shr.CreateShare(ctx, shares.CreateShareRequest{
		ShareId:       shareId,
		StaticParams:  req.Parameters,
		MutableParams: req.MutableParameters,
		LocalVolume:   strCaps.LocalPathType,
		FsPath:        volumeState.FsPath,
		Volume:        vol,
	})
	if err != nil {
		return nil, fmt.Errorf("create share: %w", err)
	}
	if shareHandle == nil {
		return nil, fmt.Errorf("create share: nil result")
	}
	err = shareHandle.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate share handle: %w", err)
	}
	logger.Info("share.CreateShare: success")
	defer func() {
		if success {
			return
		}
		logger.Info("deleting share on failure")
		logs.FromCtx(ctx).MicroOp("running share cleanup")
		err = shareHandle.DeleteShare(ctx)
		if err != nil {
			logs.FromCtx(ctx).Error("failed to run share cleanup", "error", err.Error())
			return
		}
	}()
	shareData, err := shareHandle.Serialize()
	if err != nil {
		return nil, fmt.Errorf("serialize share: %w", err)
	}

	logs.FromCtx(ctx).MicroOp("creating volume context")
	vc := volumecontext.VolumeContext{
		ServerName:    dispatchParams.ServerName,
		StorageType:   dispatchParams.StorageType,
		ShareType:     dispatchParams.ShareType,
		ConnectorId:   dispatchParams.ConnectorId,
		PresetName:    dispatchParams.NodePreset,
		Uuid:          callUuid,
		FsGroupMethod: dispatchParams.FsGroupPolicy,
	}
	err = vc.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate volume context: %w", err)
	}
	vcMap, err := serialmaps.Serialize(vc, volumecontext.Prefix)
	if err != nil {
		return nil, fmt.Errorf("serialize volume context: %w", err)
	}

	var addressIds []string
	for _, v := range addressOptions {
		addressIds = append(addressIds, v.StableId)
	}

	serializedVolume, err := vol.Serialize()
	if err != nil {
		return nil, fmt.Errorf("serialize storage volume: %w", err)
	}

	dbEntry, err := srv.dbInstance.CreateEntry(ctx, req.Name, callUuid)
	if err != nil {
		return nil, fmt.Errorf("create db entry: %w", err)
	}
	wrapped := voldbwrapper.New(dbEntry)
	// TODO save req.VolumeContentSource
	err = wrapped.SetVolumeInfo(ctx, voldbwrapper.VolumeInfo{
		ConnectorId:   dispatchParams.ConnectorId,
		VolumeContext: vc,
		AddressIds:    addressIds,
		StorageType:   dispatchParams.StorageType,
		StorageData:   serializedVolume,
		ShareType:     dispatchParams.ShareType,
		ShareData:     shareData,
		InitConfig:    initConfig,
		Size:          volumeState.Size,
		LocalPathType: strCaps.LocalPathType,
	})
	if err != nil {
		return nil, fmt.Errorf("save volume info: %w", err)
	}

	success = true
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeId,
			CapacityBytes: volumeState.Size,
			VolumeContext: vcMap,
			ContentSource: req.VolumeContentSource,
		},
	}, nil
}

func (d *Dispatcher) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	logger := logs.FromCtx(ctx)

	mh, err := d.getVolumeInfo(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return &csi.DeleteVolumeResponse{}, nil
	}

	_, err = d.unpublishVolume(ctx, mh, "")
	if err != nil {
		return nil, fmt.Errorf("unpublish before delete: %w", err)
	}

	strInstance, err := storagefactory.StorageFromType(mh.srv.trInstance, mh.vi.StorageType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	shrInstance, err := sharefactory.ShareFromType(mh.srv.trInstance, mh.vi.ShareType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	logger.MicroOp("initialized managers", "storage", mh.vi.StorageType, "share", mh.vi.ShareType)

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
	err = shareHandle.DeleteShare(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete share[%s]: %w", mh.vi.ShareType, err)
	}
	logger.Info("share.DeleteShare: success")

	vol, err := strInstance.ParseVolume(mh.vi.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	err = vol.DeleteState(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete storage[%s]: %w", mh.vi.StorageType, err)
	}
	logger.Info("storage.DeleteVolume: success")

	err = mh.wrapper.Delete(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete db entry: %w", err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

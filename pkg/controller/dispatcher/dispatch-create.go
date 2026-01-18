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
	"github.com/google/uuid"
	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/shares/sharefactory"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/storagefactory"
	"meoe.io/poly-csi/pkg/controller/voldbwrapper"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/transport"
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

	if strings.Contains(req.Name, "/") {
		return nil, fmt.Errorf("volume name must not contain '/' symbol")
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
	defer func() {
		if vol == nil {
			return
		}
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

	if req.VolumeContentSource != nil {
		var sourceSnapshot storage.Snapshot
		var sourceServer *parsedServer
		var sourceCtx context.Context

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

			sourceCtx = transport.MakeContext(ctx, sh.srv.trInstance)
			sourceServer = sh.srv

			sourceStr, err := storagefactory.StorageFromType(sh.si.StorageType, sh.srv.conf.Commands, sh.srv.conf.GlobalParams)
			if err != nil {
				return nil, fmt.Errorf("get source storage: %w", err)
			}

			snap, err := sourceStr.ParseNativeSnapshot(sh.si.SerializedData)
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

			sourceCtx = transport.MakeContext(ctx, sh.srv.trInstance)
			sourceServer = sh.srv

			sourceStr, err := storagefactory.StorageFromType(sh.vi.StorageType, sh.srv.conf.Commands, sh.srv.conf.GlobalParams)
			if err != nil {
				return nil, fmt.Errorf("get source storage: %w", err)
			}

			sourceVol, err := sourceStr.ParseVolume(sh.vi.StorageData)
			if err != nil {
				return nil, fmt.Errorf("parse source storage data: %w", err)
			}

			sourceCaps := sourceStr.GetCapabilities()
			if sourceCaps.NativeSnapshot {
				sourceSnapshot, err = sourceVol.MakeNativeSnapshot(ctx, tmpSnapshotId)
				if err != nil {
					return nil, fmt.Errorf("create source snapshot: %w", err)
				}
				// TODO cleanup should use a different context, with extended timeout
				defer sourceSnapshot.Delete(ctx)
			} else {
				return nil, fmt.Errorf("source volume does not support snapshots")
			}
		} else {
			return nil, fmt.Errorf("unknown volume content source type: %#v", req.VolumeContentSource.Type)
		}

		if sourceSnapshot == nil {
			return nil, fmt.Errorf("unexpectedly missing source snapshot data")
		}
		if sourceCtx == nil {
			return nil, fmt.Errorf("unexpectedly missing source context")
		}

		var sendFunc sendFuncType
		var receiveFunc receiveFuncType

		sourceCaps := sourceSnapshot.GetCapabilities()
		targetCaps := str.GetCapabilities()
		if sourceCaps.LocalSend != "" && targetCaps.LocalSend != "" &&
			sourceCaps.LocalSend == targetCaps.LocalSend {
			// Both source and target has send-receive support, and use the same format
			sendFunc = func(ctx context.Context, targetFile string, timeout time.Duration) error {
				return sourceSnapshot.Send(ctx, targetFile, timeout)
			}
			receiveFunc = func(ctx context.Context, sourceFile string, timeout time.Duration) (storage.Volume, error) {
				return str.Receive(ctx, sourceFile, storageCreateRequest, timeout)
			}
		} else if sourceCaps.LocalVolume != "" && targetCaps.LocalVolume != "" &&
			sourceCaps.LocalVolume == targetCaps.LocalVolume {
			// File-based volumes, can copy contents
			sourcePath, releaseSnapshot, err := sourceSnapshot.GetLocalPath(sourceCtx)
			if err != nil {
				return nil, fmt.Errorf("get snapshot local path: %w", err)
			}
			defer releaseSnapshot()

			recvVol, err := str.CreateVolume(ctx, storageCreateRequest)
			if err != nil {
				return nil, fmt.Errorf("storage[%s]:CreateVolume: %w", dispatchParams.storageType, err)
			}
			targetPath, err := recvVol.GetLocalPath(ctx)
			if err != nil {
				return nil, fmt.Errorf("storage[%s]:CreateVolume:GetLocalPath %w", dispatchParams.storageType, err)
			}
			logger.Info("storage.CreateVolume: success")

			switch sourceCaps.LocalVolume {
			case storage.VolFolder:
				sendFunc = func(ctx context.Context, targetFile string, timeout time.Duration) error {
					return sendTar(ctx, sourcePath, targetFile, timeout)
				}
				receiveFunc = func(ctx context.Context, sourceFile string, timeout time.Duration) (storage.Volume, error) {
					err := receiveTar(ctx, sourceFile, targetPath, timeout)
					if err != nil {
						return nil, fmt.Errorf("receive tar: %w", err)
					}
					return recvVol, nil
				}
			case storage.VolDevice:
				sendFunc = func(ctx context.Context, targetFile string, timeout time.Duration) error {
					return sendFile(ctx, sourcePath, targetFile, timeout)
				}
				receiveFunc = func(ctx context.Context, sourceFile string, timeout time.Duration) (storage.Volume, error) {
					err := receiveFile(ctx, sourceFile, targetPath, timeout)
					if err != nil {
						return nil, fmt.Errorf("receive device file: %w", err)
					}
					return recvVol, nil
				}
			default:
				return nil, fmt.Errorf("unknown local volume type: sourceCaps.LocalVolume")
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

		sendCtx, sendCancel := context.WithTimeout(sourceCtx, timeout)
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
			sc, err := receiveFunc(receiveCtx, receivePipe, timeout)
			vol = sc
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
				return nil, fmt.Errorf("receive: %w", err)
			}
			logger.Info("storage.Receive: success")
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
	volumeContext["uuid"] = uuidString

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

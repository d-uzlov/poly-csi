package dispatcher

import (
	"bytes"
	"context"
	"fmt"
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
	K8sNs       string
	K8sPvc      string
}

func generateId(tmplName string, tmpl string, requestName string, staticParams map[string]string) (string, error) {
	t, err := template.New(tmplName).Parse(tmpl)
	if err != nil {
		return "", err
	}

	buf := bytes.Buffer{}
	err = t.Execute(&buf, IdFields{
		RequestName: requestName,
		K8sNs:       staticParams["csi.storage.k8s.io/pvc/namespace"],
		K8sPvc:      staticParams["csi.storage.k8s.io/pvc/name"],
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

type dispatchCreate struct {
	rawName          string
	rawStaticParams  map[string]string
	rawMutableParams map[string]string
	size             int64
	p                DispatchParams
	// j       DispatchParams

	volCapsFs *csi.VolumeCapability_MountVolume
	init      volumeinit.InitConfig

	srv        *parsedServer
	srvRelease func()

	str     storage.StorageManager
	volMain storage.Volume

	shr            shares.ShareManager
	shareHandle    shares.Share
	shareViaVolume bool

	addressOptions []AddressOption
	addressIds     []string

	volumeId string
}

func (dc *dispatchCreate) release() {
	if dc.srvRelease != nil {
		dc.srvRelease()
	}
}

func (dc *dispatchCreate) parseParams(req *csi.CreateVolumeRequest) error {
	if strings.Contains(req.Name, "/") {
		return fmt.Errorf("volume name must not contain '/' symbol")
	}

	dc.rawName = req.Name

	dc.size = req.CapacityRange.RequiredBytes

	dispatchParams, err := parseDispatchParams(nil, req.Parameters, req.MutableParameters)
	if err != nil {
		return fmt.Errorf("parseDispatchParams: %w", err)
	}
	dc.p = dispatchParams

	// journalParams := dispatchParams
	// err = serialmaps.ParseRemove(req.Parameters, paramsJournalPrefix, true, &journalParams)
	// if err != nil {
	// 	return fmt.Errorf("parse journal params: %w", err)
	// }
	// dc.j = journalParams

	var initConfig volumeinit.InitConfig
	err = serialmaps.ParseRemove(req.Parameters, volumeinit.PreferredPrefix, true, &initConfig)
	if err != nil {
		return fmt.Errorf("parse init config: %w", err)
	}
	err = initConfig.Validate()
	if err != nil {
		return fmt.Errorf("validate init config: %w", err)
	}

	if len(req.VolumeCapabilities) == 0 {
		return fmt.Errorf("request does not define volume capabilities")
	}

	dc.volCapsFs = req.VolumeCapabilities[0].GetMount()

	dc.init = initConfig
	if dc.volCapsFs != nil {
		dc.init.FsType = dc.volCapsFs.FsType
	}

	dc.rawStaticParams = req.Parameters
	dc.rawMutableParams = req.MutableParameters

	return nil
}

func (dc *dispatchCreate) initStorageHandles(findServer func(name string) (result *parsedServer, err error)) error {
	srv, err := findServer(dc.p.ServerName)
	if err != nil {
		return fmt.Errorf("server[name=%s]: not found", dc.p.ServerName)
	}
	srv.mutex.RLock()
	dc.srv = srv
	dc.srvRelease = srv.mutex.RUnlock

	err = srv.Check()
	if err != nil {
		return fmt.Errorf("server[name=%s]: %w", dc.p.ServerName, err)
	}

	dc.str, err = storagefactory.StorageFromType(srv.trInstance, dc.p.StorageType, srv.Conf.GlobalParams)
	if err != nil {
		return err
	}

	dc.shr, err = sharefactory.ShareFromType(srv.trInstance, dc.p.ShareType, srv.Conf.GlobalParams)
	if err != nil {
		return err
	}

	dc.volumeId = createVolumeId(srv.Conf.StableId, dc.rawName)

	for name := range strings.SplitSeq(dc.p.AddressNames, ",") {
		found := false
		for _, v := range srv.Conf.AddressList {
			if v.Name == name {
				dc.addressOptions = append(dc.addressOptions, v)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("address '%s' is not found", name)
		}
	}

	for _, v := range dc.addressOptions {
		dc.addressIds = append(dc.addressIds, v.StableId)
	}

	strCaps := dc.str.GetCapabilities()
	shrCaps := dc.shr.GetCapabilities()

	if strCaps.LocalPathType == "" && strCaps.OpaqueVolumeType == "" {
		return fmt.Errorf("storage reported empty both LocalPathType and OpaqueVolumeType")
	}

	if shrCaps.LocalVolume != "" && shrCaps.LocalVolume != strCaps.LocalPathType {
		return fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", dc.p.ShareType, shrCaps.LocalVolume, dc.p.StorageType, strCaps.LocalPathType)
	}

	if shrCaps.OpaqueVolume != "" {
		if shrCaps.OpaqueVolume == strCaps.OpaqueVolumeType {
			return fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", dc.p.ShareType, shrCaps.OpaqueVolume, dc.p.StorageType, strCaps.OpaqueVolumeType)
		}
		dc.shareViaVolume = true
	}

	storageId := dc.rawName
	if dc.p.StorageIdTemplate != "" {
		storageId, err = generateId("storageIdTemplate", dc.p.StorageIdTemplate, dc.rawName, dc.rawStaticParams)
		if err != nil {
			return fmt.Errorf("generate storage id: %w", err)
		}
	}

	dc.volMain, err = dc.str.CreateVolume(storage.CreateRequest{
		Id:            storageId,
		StaticParams:  dc.rawStaticParams,
		MutableParams: dc.rawMutableParams,
	})
	if err != nil {
		return fmt.Errorf("storage[%s]:CreateVolume: %w", dc.p.StorageType, err)
	}
	if dc.volMain == nil {
		return fmt.Errorf("storage[%s]:CreateVolume: nil result", dc.p.StorageType)
	}

	shareId := dc.rawName
	if dc.p.ShareIdTemplate != "" {
		id, err := generateId("shareIdTemplate", dc.p.ShareIdTemplate, dc.rawName, dc.rawStaticParams)
		if err != nil {
			return fmt.Errorf("generate share id: %w", err)
		}
		shareId = id
	}

	dc.shareHandle, err = dc.shr.CreateShare(shares.CreateShareRequest{
		ShareId:       shareId,
		StaticParams:  dc.rawStaticParams,
		MutableParams: dc.rawMutableParams,
	})
	if err != nil {
		return fmt.Errorf("create share: %w", err)
	}
	if dc.shareHandle == nil {
		return fmt.Errorf("create share: nil result")
	}
	err = dc.shareHandle.Validate()
	if err != nil {
		return fmt.Errorf("validate share handle: %w", err)
	}

	return nil
}

func (dc *dispatchCreate) createFromScratch(ctx context.Context) error {
	err := dc.volMain.CreateState(ctx, dc.size)
	if err != nil {
		return fmt.Errorf("CreateState: %w", err)
	}
	logs.FromCtx(ctx).Info("storage.CreateVolume: success")
	return nil
}

type cloneSource struct {
	srv *parsedServer

	str  storage.StorageManager
	snap storage.Snapshot
}

func (dc *dispatchCreate) getExistingSnapshotForClone(ctx context.Context, sh *snapshotManagementHandle) (*cloneSource, error) {
	sourceStr, err := storagefactory.StorageFromType(sh.srv.trInstance, sh.si.StorageType, sh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, fmt.Errorf("get source storage: %w", err)
	}

	snap, err := sourceStr.ParseSnapshot(sh.si.SerializedData)
	if err != nil {
		return nil, fmt.Errorf("parse source snapshot data: %w", err)
	}

	return &cloneSource{
		srv:  sh.srv,
		str:  sourceStr,
		snap: snap,
	}, nil
}

func (dc *dispatchCreate) makeSnapshotForClone(ctx context.Context, sh *volumeManagementHandle) (*cloneSource, error) {
	sourceStr, err := storagefactory.StorageFromType(sh.srv.trInstance, sh.vi.Storage.StorageType, sh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, fmt.Errorf("get source storage: %w", err)
	}

	sourceVol, err := sourceStr.ParseVolume(sh.vi.Storage.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse source storage data: %w", err)
	}

	sourceCaps := sourceStr.GetCapabilities()
	if !sourceCaps.NativeSnapshot {
		return nil, fmt.Errorf("source volume does not support native snapshots")
	}

	tmpSnapshotId := "snap-for-" + dc.rawName
	sourceSnapshot, err := sourceVol.MakeSnapshot(ctx, tmpSnapshotId)
	if err != nil {
		return nil, fmt.Errorf("create source snapshot: %w", err)
	}
	// TODO cleanup should use a different context, with extended timeout
	defer sourceSnapshot.Delete(ctx)

	return &cloneSource{
		srv:  sh.srv,
		str:  sourceStr,
		snap: sourceSnapshot,
	}, nil
}

func (dc *dispatchCreate) createFromSnapshot(ctx context.Context, source cloneSource) error {
	logger := logs.FromCtx(ctx)

	var sendFunc sendFuncType
	var receiveFunc receiveFuncType

	sourceCaps := source.str.GetCapabilities()
	targetCaps := dc.str.GetCapabilities()
	if sourceCaps.LocalSendType != "" && targetCaps.LocalSendType != "" &&
		sourceCaps.LocalSendType == targetCaps.LocalSendType {
		// Both source and target has send-receive support, and use the same format
		sendFunc = func(ctx context.Context, targetFile string, timeout time.Duration) error {
			return source.snap.Send(ctx, targetFile, timeout)
		}
		receiveFunc = func(ctx context.Context, sourceFile string, timeout time.Duration) error {
			return dc.volMain.ReceiveSnapshot(ctx, sourceFile, timeout)
		}
	} else if sourceCaps.LocalPathType != "" && targetCaps.LocalPathType != "" &&
		sourceCaps.LocalPathType == targetCaps.LocalPathType {
		// File-based volumes, can copy contents
		sourcePath, releaseSnapshot, err := source.snap.GetLocalPath(ctx)
		if err != nil {
			return fmt.Errorf("get snapshot local path: %w", err)
		}
		defer releaseSnapshot()

		err = dc.volMain.CreateState(ctx, dc.size)
		if err != nil {
			return fmt.Errorf("CreateState: %w", err)
		}
		volumeState, err := dc.volMain.ReadState(ctx)
		if err != nil {
			return fmt.Errorf("storage[%s]:CreateVolume:ReadState: %w", dc.p.StorageType, err)
		}
		if volumeState == nil {
			return fmt.Errorf("storage[%s]:CreateVolume:ReadState: nil result", dc.p.StorageType)
		}
		logger.Info("storage.CreateVolume: success")

		switch sourceCaps.LocalPathType {
		case storage.VolFolder:
			sendFunc = func(ctx context.Context, targetFile string, timeout time.Duration) error {
				return sendTar(ctx, source.srv.trInstance, sourcePath, targetFile, timeout)
			}
			receiveFunc = func(ctx context.Context, sourceFile string, timeout time.Duration) error {
				err := receiveTar(ctx, dc.srv.trInstance, sourceFile, volumeState.FsPath, timeout)
				if err != nil {
					return fmt.Errorf("receive tar: %w", err)
				}
				return nil
			}
		case storage.VolDevice:
			sendFunc = func(ctx context.Context, targetFile string, timeout time.Duration) error {
				return sendFile(ctx, source.srv.trInstance, sourcePath, targetFile, timeout)
			}
			receiveFunc = func(ctx context.Context, sourceFile string, timeout time.Duration) error {
				err := receiveFile(ctx, dc.srv.trInstance, sourceFile, volumeState.FsPath, timeout)
				if err != nil {
					return fmt.Errorf("receive device file: %w", err)
				}
				return nil
			}
		default:
			return fmt.Errorf("unknown local volume type: %s", sourceCaps.LocalPathType)
		}
	} else {
		return fmt.Errorf("source and target use incompatible send-receive mechanism")
	}

	// TODO better timeout
	timeout := time.Minute

	pipeName := "send-for-" + dc.rawName
	sendPipe, receivePipe, release, err := createClonePipes(ctx, pipeName, source.srv, dc.srv, timeout)
	if release != nil {
		defer release()
	}
	if err != nil {
		return fmt.Errorf("createClonePipes: %w", err)
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
		return fmt.Errorf("wait send: %w", ctx.Err())
	case err = <-sendCh:
		if err != nil {
			return fmt.Errorf("send: %w", err)
		}
		logger.Info("storage.Send: success")
		// send OK, now let's check receive
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("wait receive: %w", ctx.Err())
	case err = <-receiveCh:
		if err != nil {
			return fmt.Errorf("storage[%s]:receive: %w", dc.p.StorageType, err)
		}
		logger.Info("storage.Receive: success")
	}

	return nil
}

func (d *Dispatcher) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (resp *csi.CreateVolumeResponse, finalErr error) {
	logger := logs.FromCtx(ctx)

	dc := dispatchCreate{}
	defer dc.release()
	err := dc.parseParams(req)
	if err != nil {
		return nil, fmt.Errorf("parseParams: %w", err)
	}

	err = dc.initStorageHandles(d.FindServerByName)
	if err != nil {
		return nil, fmt.Errorf("initStorageHandles: %w", err)
	}

	callUuid := uuidctx.FromCtx(ctx)

	dbEntry, err := dc.srv.dbInstance.CreateEntry(ctx, dc.rawName, callUuid)
	if err != nil {
		return nil, fmt.Errorf("create db entry: %w", err)
	}
	wrapped := voldbwrapper.New(dbEntry)

	vi, err := wrapped.GetVolumeInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	if vi == nil {
		vi = &voldbwrapper.VolumeInfo{}
	}

	success := false

	if vi.Storage.DataSource == "" {
		if req.VolumeContentSource != nil {
			var clone *cloneSource
			if src := req.VolumeContentSource.GetSnapshot(); src != nil {
				sh, err := d.getSnapshotHandle(ctx, src.SnapshotId)
				if err != nil {
					return nil, fmt.Errorf("getSnapshotHandle: %w", err)
				}
				defer sh.release()
				if sh.si == nil {
					return nil, fmt.Errorf("source snapshot is not found")
				}
				clone, err = dc.getExistingSnapshotForClone(ctx, sh)
				if err != nil {
					return nil, fmt.Errorf("getExistingSnapshotForClone: %w", err)
				}
				vi.Storage.DataSource = src.SnapshotId
			} else if src := req.VolumeContentSource.GetVolume(); src != nil {
				sh, err := d.getExistingVolumeHandle(ctx, src.VolumeId)
				if err != nil {
					return nil, fmt.Errorf("get volume info: %w", err)
				}
				defer sh.release()
				if sh.vi == nil {
					return nil, fmt.Errorf("source volume is not found")
				}

				clone, err = dc.makeSnapshotForClone(ctx, sh)
				if err != nil {
					return nil, fmt.Errorf("makeSnapshotForClone: %w", err)
				}
				vi.Storage.DataSource = src.VolumeId
			} else {
				return nil, fmt.Errorf("unknown volume content source type: %#v", req.VolumeContentSource.Type)
			}
			if clone == nil {
				return nil, fmt.Errorf("get source snapshot: nil result")
			}
			err = dc.createFromSnapshot(ctx, *clone)
			if err != nil {
				return nil, fmt.Errorf("createFromSnapshot: %w", err)
			}
		} else {
			err = dc.createFromScratch(ctx)
			if err != nil {
				return nil, fmt.Errorf("createFromScratch: %w", err)
			}
			vi.Storage.NeedsControllerFormat = true
			vi.Storage.DataSource = "scratch"
		}
	}
	vi.Storage.StorageType = dc.p.StorageType
	vi.Storage.StorageData, err = dc.volMain.Serialize()
	if err != nil {
		return nil, fmt.Errorf("volMain.Serialize: %w", err)
	}
	err = wrapped.SetVolumeInfo(ctx, *vi)
	if err != nil {
		return nil, fmt.Errorf("SetVolumeInfo: %w", err)
	}

	strCaps := dc.str.GetCapabilities()
	volumeState, err := dc.volMain.ReadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if volumeState == nil {
		return nil, fmt.Errorf("read state: nil result")
	}
	if dc.str.GetCapabilities().LocalPathType != "" {
		if volumeState.FsPath == "" {
			return nil, fmt.Errorf("storage did not set FsPath")
		}
		// when using local volumes, we can check common stuff here, to avoid repeating it in each share
		err = checkFile(ctx, dc.srv.trInstance, strCaps.LocalPathType, volumeState.FsPath)
		if err != nil {
			return nil, fmt.Errorf("check local file: %w", err)
		}
	}
	if volumeState.Size < req.CapacityRange.RequiredBytes {
		err = dc.volMain.SetSize(ctx, req.CapacityRange.RequiredBytes)
		if err != nil {
			return nil, fmt.Errorf("SetSize: %w", err)
		}
		volumeState, err = dc.volMain.ReadState(ctx)
		if err != nil {
			return nil, fmt.Errorf("read state: %w", err)
		}
	}

	vi.Storage.Size = volumeState.Size
	vi.Storage.LocalPathType = strCaps.LocalPathType

	if dc.volCapsFs == nil ||
		strCaps.LocalPathType != storage.VolDevice ||
		dc.p.FsFormatSide != FsFormatController {
		vi.Storage.NeedsControllerFormat = false
	}
	if vi.Storage.NeedsControllerFormat {
		if volumeState.FsPath == "" {
			return nil, fmt.Errorf("read state: empty fs path")
		}

		dc.init.InitialEmpty = strCaps.BlockInitialEmpty
		dc.init.AtomicSize = volumeState.AtomicSize
		dc.init.RMWSize = volumeState.RMWSize
		dc.init.OptimalWriteSize = volumeState.OptimalWriteSize

		for k := range req.Parameters {
			if strings.HasPrefix(k, volumeinit.PreferredPrefix) {
				return nil, fmt.Errorf("unknown key '%s'", k)
			}
		}

		init := volumeinit.NewController(dc.srv.trInstance)
		logs.FromCtx(ctx).MicroOp("running volume init", "volume-type", strCaps.LocalPathType)
		_, err = init.RunBlock(ctx, dc.init, volumeState.FsPath)
		if err != nil {
			return nil, fmt.Errorf("init block: %w", err)
		}
		vi.Storage.NeedsControllerFormat = false
	}

	vi.InitConfig = dc.init

	err = wrapped.SetVolumeInfo(ctx, *vi)
	if err != nil {
		return nil, fmt.Errorf("SetVolumeInfo: %w", err)
	}

	err = dc.shareHandle.PrepareShare(ctx)
	if err != nil {
		return nil, fmt.Errorf("PrepareShare: %w", err)
	}
	if dc.shareViaVolume {
		err = dc.shareHandle.AttachVolume(ctx, dc.volMain, 0)
		if err != nil {
			return nil, fmt.Errorf("ShareVolume: %w", err)
		}
	} else {
		err = dc.shareHandle.AttachPath(ctx, volumeState.FsPath, 0)
		if err != nil {
			return nil, fmt.Errorf("SharePath: %w", err)
		}
	}

	logger.Info("share.CreateShare: success")
	defer func() {
		if success {
			return
		}
		logger.Info("deleting share on failure")
		logs.FromCtx(ctx).MicroOp("running share cleanup")
		err = dc.shareHandle.DeleteShare(ctx)
		if err != nil {
			logs.FromCtx(ctx).Error("failed to run share cleanup", "error", err.Error())
			return
		}
	}()
	vi.ShareType = dc.p.ShareType
	vi.ShareData, err = dc.shareHandle.Serialize()
	if err != nil {
		return nil, fmt.Errorf("serialize share: %w", err)
	}

	vi.VolumeContext = volumecontext.VolumeContext{
		Uuid:          callUuid,
		ServerName:    dc.p.ServerName,
		StorageType:   dc.p.StorageType,
		ShareType:     dc.p.ShareType,
		ConnectorId:   dc.p.ConnectorId,
		PresetName:    dc.p.NodePreset,
		FsGroupMethod: dc.p.FsGroupPolicy,
		// TODO customize
		FsRootDirMode: "777",
	}
	err = vi.VolumeContext.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate volume context: %w", err)
	}
	vcMap, err := serialmaps.Serialize(vi.VolumeContext, volumecontext.Prefix)
	if err != nil {
		return nil, fmt.Errorf("serialize volume context: %w", err)
	}

	vi.ConnectorId = dc.p.ConnectorId
	vi.AddressIds = dc.addressIds

	err = wrapped.SetVolumeInfo(ctx, *vi)
	if err != nil {
		return nil, fmt.Errorf("SetVolumeInfo: %w", err)
	}

	success = true
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      dc.volumeId,
			CapacityBytes: volumeState.Size,
			VolumeContext: vcMap,
			ContentSource: req.VolumeContentSource,
		},
	}, nil
}

func (d *Dispatcher) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	logger := logs.FromCtx(ctx)

	mh, err := d.getExistingVolumeHandle(ctx, req.VolumeId)
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

	strInstance, err := storagefactory.StorageFromType(mh.srv.trInstance, mh.vi.Storage.StorageType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	shrInstance, err := sharefactory.ShareFromType(mh.srv.trInstance, mh.vi.ShareType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	logger.MicroOp("initialized managers", "storage", mh.vi.Storage.StorageType, "share", mh.vi.ShareType)

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

	vol, err := strInstance.ParseVolume(mh.vi.Storage.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	err = vol.DeleteState(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete storage[%s]: %w", mh.vi.Storage.StorageType, err)
	}
	logger.Info("storage.DeleteVolume: success")

	err = mh.wrapper.Delete(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete db entry: %w", err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

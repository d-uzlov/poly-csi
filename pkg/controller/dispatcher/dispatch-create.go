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
	rawName              string
	rawStaticParams      map[string]string
	rawMutableParams     map[string]string
	journalStaticParams  map[string]string
	journalMutableParams map[string]string
	size                 int64
	p                    DispatchParams
	j                    DispatchParams

	volCapsFs *csi.VolumeCapability_MountVolume

	srv        *parsedServer
	srvRelease func()

	fs volumeinit.FilesystemHelper

	str     storage.StorageManager
	volMain storage.Volume

	strJ       storage.StorageManager
	volJournal storage.Volume

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

	dispatchParams, err := parseDispatchParams(paramsPrefix, nil, req.Parameters, req.MutableParameters)
	if err != nil {
		return fmt.Errorf("parseDispatchParams: %w", err)
	}
	dc.p = dispatchParams

	journalParams := dispatchParams
	err = serialmaps.ParseRemove(req.Parameters, paramsJournalPrefix, true, &journalParams)
	if err != nil {
		return fmt.Errorf("parse journal params: %w", err)
	}
	dc.j = journalParams

	if len(req.VolumeCapabilities) == 0 {
		return fmt.Errorf("request does not define volume capabilities")
	}

	dc.volCapsFs = req.VolumeCapabilities[0].GetMount()
	if dc.volCapsFs != nil && dc.volCapsFs.FsType != "" {
		dc.fs = volumeinit.GetFromType(dc.volCapsFs.FsType)
		if dc.fs == nil {
			return fmt.Errorf("filesystem handler for fs type '%s' is not found", dc.volCapsFs.FsType)
		}
		dc.fs.Configure(dc.rawStaticParams)

		caps := dc.fs.Capabilities()
		if dc.size < caps.MinFsSize {
			dc.size = caps.MinFsSize
		}
	}

	dc.rawStaticParams = req.Parameters
	dc.rawMutableParams = req.MutableParameters

	dc.journalStaticParams = map[string]string{}
	dc.journalMutableParams = map[string]string{}

	for k, v := range dc.rawStaticParams {
		kk, ok := strings.CutPrefix(k, paramsJournalPrefix)
		if ok {
			dc.journalStaticParams[kk] = v
		}
	}

	for k, v := range dc.rawMutableParams {
		kk, ok := strings.CutPrefix(k, paramsJournalPrefix)
		if ok {
			dc.journalMutableParams[kk] = v
		}
	}

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

	dc.strJ, err = storagefactory.StorageFromType(srv.trInstance, dc.j.StorageType, srv.Conf.GlobalParams)
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
	strJCaps := dc.strJ.GetCapabilities()
	shrCaps := dc.shr.GetCapabilities()

	// storage-to-share checks
	if strCaps.LocalPathType == "" && strCaps.OpaqueType == "" {
		return fmt.Errorf("storage reported empty for both LocalPathType and OpaqueVolumeType")
	}

	if shrCaps.LocalVolume != "" && shrCaps.LocalVolume != strCaps.LocalPathType {
		return fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", dc.p.ShareType, shrCaps.LocalVolume, dc.p.StorageType, strCaps.LocalPathType)
	}

	if shrCaps.OpaqueVolume != "" {
		if shrCaps.OpaqueVolume != strCaps.OpaqueType {
			return fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", dc.p.ShareType, shrCaps.OpaqueVolume, dc.p.StorageType, strCaps.OpaqueType)
		}
		dc.shareViaVolume = true
	}

	// journal-to-share checks

	if dc.p.JournalVolume != "none" {
		if !shrCaps.MultiShare {
			return fmt.Errorf("share '%s' does not support multiple volumes", dc.p.ShareType)
		}

		if strJCaps.LocalPathType == "" && strJCaps.OpaqueType == "" {
			return fmt.Errorf("journal storage reported empty for both LocalPathType and OpaqueVolumeType")
		}

		if shrCaps.LocalVolume != "" && shrCaps.LocalVolume != strJCaps.LocalPathType {
			return fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", dc.p.ShareType, shrCaps.LocalVolume, dc.j.StorageType, strCaps.LocalPathType)
		}

		if shrCaps.OpaqueVolume != "" {
			if shrCaps.OpaqueVolume == strJCaps.OpaqueType {
				return fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", dc.p.ShareType, shrCaps.OpaqueVolume, dc.j.StorageType, strCaps.OpaqueType)
			}
			dc.shareViaVolume = true
		}
	}

	// main volume
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

	// journal volume
	if dc.p.JournalVolume != "none" {
		storageIdJ := dc.rawName
		if dc.j.StorageIdTemplate != "" {
			storageIdJ, err = generateId("storageIdTemplate(journal)", dc.j.StorageIdTemplate, dc.rawName, dc.rawStaticParams)
			if err != nil {
				return fmt.Errorf("generate journal storage id: %w", err)
			}
		}
		if storageIdJ == storageId {
			return fmt.Errorf("main_storage_id == journal_storage_id")
		}

		dc.volJournal, err = dc.strJ.CreateVolume(storage.CreateRequest{
			Id:            storageIdJ,
			StaticParams:  dc.journalStaticParams,
			MutableParams: dc.journalMutableParams,
		})
		if err != nil {
			return fmt.Errorf("storage[%s]:CreateVolume(journal): %w", dc.j.StorageType, err)
		}
		if dc.volJournal == nil {
			return fmt.Errorf("storage[%s]:CreateVolume(journal): nil result", dc.j.StorageType)
		}
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

	switch dc.p.JournalVolume {
	case "none":
	case "sameShare":
		if !dc.shr.GetCapabilities().MultiShare {
			return fmt.Errorf("can't use journalVolume=sameShare because share '%s' does not support multiple volumes", dc.p.ShareType)
		}
	default:
		return fmt.Errorf("unsupported JournalVolume: '%s'", dc.p.JournalVolume)
	}

	return nil
}

func (dc *dispatchCreate) createFromScratch(ctx context.Context) error {
	err := dc.volMain.CreateState(ctx, dc.size)
	if err != nil {
		return fmt.Errorf("CreateState: %w", err)
	}
	if dc.p.JournalVolume != "none" {
		if dc.fs == nil {
			return fmt.Errorf("trying to make journal without a filesystem helper")
		}
		size := dc.fs.CalcPreferredJournalSize(dc.size)
		err = dc.volJournal.CreateState(ctx, size)
		if err != nil {
			return fmt.Errorf("CreateState(journal): %w", err)
		}
	}
	logs.FromCtx(ctx).Info("storage.CreateState: success")
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
	if dc.p.JournalVolume != "none" {
		vi.Storage.JournalStorageType = dc.j.StorageType
		vi.Storage.JournalStorageData, err = dc.volJournal.Serialize()
		if err != nil {
			return nil, fmt.Errorf("volMain.Serialize: %w", err)
		}
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
	// TODO do we even need this check?
	if volumeState.Size < dc.size {
		err = dc.volMain.SetSize(ctx, dc.size)
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

	vi.VolumeContext = volumecontext.VolumeContext{
		Uuid:          callUuid,
		ServerName:    dc.p.ServerName,
		StorageType:   dc.p.StorageType,
		ShareType:     dc.p.ShareType,
		ConnectorId:   dc.p.ConnectorId,
		PresetName:    dc.p.NodePreset,
		FsGroupMethod: dc.p.FsGroupPolicy,
		// TODO customize
		FsRootDirMode: "2777",
	}

	switch dc.p.JournalVolume {
	case "none":
		vi.VolumeContext.JournalVolume = "none"
	case "sameShare":
		vi.VolumeContext.JournalVolume = "sameShare"
	default:
		return nil, fmt.Errorf("unsupported JournalVolume type: '%s'", dc.p.JournalVolume)
	}

	journalProps := volumeinit.StorageProperties{}
	var journalState *storage.VolumeState
	if dc.volCapsFs != nil && dc.volCapsFs.FsType != "" {
		if dc.p.JournalVolume != "none" {
			journalState, err = dc.volJournal.ReadState(ctx)
			if err != nil {
				return nil, fmt.Errorf("read journal state: %w", err)
			}
			if journalState == nil {
				return nil, fmt.Errorf("read journal state: nil result")
			}
			if dc.strJ.GetCapabilities().LocalPathType != "" {
				if journalState.FsPath == "" {
					return nil, fmt.Errorf("storage did not set journal FsPath")
				}
				// when using local volumes, we can check common stuff here, to avoid repeating it in each share
				err = checkFile(ctx, dc.srv.trInstance, strCaps.LocalPathType, journalState.FsPath)
				if err != nil {
					return nil, fmt.Errorf("check journal local file: %w", err)
				}
			}

			journalProps = volumeinit.StorageProperties{
				InitialEmpty:     dc.strJ.GetCapabilities().BlockInitialEmpty,
				RMWSize:          journalState.RMWSize,
				OptimalWriteSize: journalState.OptimalWriteSize,
				AtomicSize:       journalState.AtomicSize,
			}
		}
		dc.fs.SetVolumeProps(volumeinit.StorageProperties{
			InitialEmpty:     strCaps.BlockInitialEmpty,
			RMWSize:          volumeState.RMWSize,
			OptimalWriteSize: volumeState.OptimalWriteSize,
			AtomicSize:       volumeState.AtomicSize,
		}, journalProps)
		vi.VolumeContext.FsType = dc.volCapsFs.FsType
		vi.VolumeContext.FsData, err = dc.fs.Serialize()
		if err != nil {
			return nil, fmt.Errorf("format.Serialize: %w", err)
		}

		if dc.volCapsFs == nil ||
			strCaps.LocalPathType != storage.VolDevice ||
			dc.p.FsFormatSide != FsFormatController {
			vi.Storage.NeedsControllerFormat = false
		}
		if vi.Storage.NeedsControllerFormat {
			if volumeState.FsPath == "" {
				return nil, fmt.Errorf("read state: empty fs path")
			}

			if dc.p.JournalVolume != "none" {
				dc.fs.SetWithJournal(volumeState.FsPath, journalState.FsPath)
			} else {
				dc.fs.SetSingleDevice(volumeState.FsPath)
			}

			err = dc.fs.Format(ctx, dc.srv.trInstance)
			if err != nil {
				return nil, fmt.Errorf("run format: %w", err)
			}
			vi.Storage.NeedsControllerFormat = false
		}
	}

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
			return nil, fmt.Errorf("AttachVolume: %w", err)
		}
	} else {
		err = dc.shareHandle.AttachPath(ctx, volumeState.FsPath, 0)
		if err != nil {
			return nil, fmt.Errorf("AttachPath: %w", err)
		}
	}
	switch dc.p.JournalVolume {
	case "none":
	case "sameShare":
		if dc.shareViaVolume {
			err = dc.shareHandle.AttachVolume(ctx, dc.volJournal, 1)
			if err != nil {
				return nil, fmt.Errorf("AttachVolume(journal): %w", err)
			}
		} else {
			err = dc.shareHandle.AttachPath(ctx, journalState.FsPath, 1)
			if err != nil {
				return nil, fmt.Errorf("AttachPath(journal): %w", err)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported JournalVolume: '%s'", dc.p.JournalVolume)
	}

	logger.Info("share.CreateShare: success")
	defer func() {
		// TODO change cleanup to use dispatchCreate struct
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
	// TODO save intermediate deletion results, to avoid double deletion ??
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

	shrInstance, err := sharefactory.ShareFromType(mh.srv.trInstance, mh.vi.ShareType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
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
	err = shareHandle.DeleteShare(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete share[%s]: %w", mh.vi.ShareType, err)
	}
	logger.Info("share.DeleteShare: success")

	if mh.vi.Storage.JournalStorageType != "" {
		journalStorage, err := storagefactory.StorageFromType(mh.srv.trInstance, mh.vi.Storage.JournalStorageType, mh.srv.Conf.GlobalParams)
		if err != nil {
			return nil, err
		}

		vol, err := journalStorage.ParseVolume(mh.vi.Storage.JournalStorageData)
		if err != nil {
			return nil, fmt.Errorf("parse journal storage data: %w", err)
		}
		if vol == nil {
			return nil, fmt.Errorf("parse journal storage data: nil result")
		}

		err = vol.DeleteState(ctx)
		if err != nil {
			return nil, fmt.Errorf("delete storage[%s](journal ): %w", mh.vi.Storage.StorageType, err)
		}
	}

	strInstance, err := storagefactory.StorageFromType(mh.srv.trInstance, mh.vi.Storage.StorageType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}
	vol, err := strInstance.ParseVolume(mh.vi.Storage.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}
	if vol == nil {
		return nil, fmt.Errorf("parse storage data: nil result")
	}

	err = vol.DeleteState(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete storage[%s]: %w", mh.vi.Storage.StorageType, err)
	}
	logger.Info("vol.DeleteState: success")

	err = mh.wrapper.Delete(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete db entry: %w", err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

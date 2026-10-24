package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/api/publishcontext"
	"meoe.io/poly-csi/pkg/api/volumecontext"
	"meoe.io/poly-csi/pkg/controller/blockshare"
	"meoe.io/poly-csi/pkg/controller/blockshare/blkshrfactory"
	"meoe.io/poly-csi/pkg/controller/blockvolume"
	"meoe.io/poly-csi/pkg/controller/blockvolume/blkvolfactory"
	"meoe.io/poly-csi/pkg/controller/voldb"
	"meoe.io/poly-csi/pkg/controller/voldbwrapper"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/filecon"
	"meoe.io/poly-csi/pkg/node/filecon/fsfactory"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type DispatchBlockFs struct {
	conf *DispatcherConf

	srv *parsedServer

	rawName string
	uuid    string

	rawStaticParams  map[string]string
	rawMutableParams map[string]string

	dbEntry voldb.Entry

	FsType string
	FsMeta filecon.Metadata

	p DispatchParams

	Block map[string]BlockInfo
}

var _ VolumeDriver = &DispatchBlockFs{}

type BlockInfo struct {
	Req            filecon.BlockDeviceRequirement
	storageManager blockvolume.Manager
	storageHandle  blockvolume.Volume
	shareManager   blockshare.Manager
	shareHandle    blockshare.Share
	p              DispatchParams
	publishData    map[string]json.RawMessage
	addrOpt        []AddressOption
}

func NewDispatchBlockFs(conf *DispatcherConf, uuid string) VolumeDriver {
	return &DispatchBlockFs{
		conf:  conf,
		uuid:  uuid,
		Block: map[string]BlockInfo{},
	}
}

func (df *DispatchBlockFs) MakeVolumeId() string {
	return createVolumeId(df.srv.Conf.StableId, df.rawName)
}

func (df *DispatchBlockFs) ParseRequest(req *csi.CreateVolumeRequest) error {
	df.rawName = req.Name

	fsCaps := req.VolumeCapabilities[0].GetMount()
	if fsCaps.FsType == "" {
		return fmt.Errorf("missing fs type")
	}
	df.FsType = fsCaps.FsType

	fsInfo, err := fsfactory.GetByType(df.FsType)
	if err != nil {
		return fmt.Errorf("get filesystem metadata: %w", err)
	}

	df.rawStaticParams = req.Parameters
	df.rawMutableParams = req.MutableParameters

	dispatchParams, err := parseDispatchParams(paramsPrefix, nil, req.Parameters, req.MutableParameters)
	if err != nil {
		return fmt.Errorf("parseDispatchParams: %w", err)
	}
	df.p = dispatchParams

	if fsInfo.Meta == nil {
		return fmt.Errorf("fsInfo.Meta == nil")
	}

	df.FsMeta = fsInfo.Meta()
	err = df.FsMeta.Configure(req.CapacityRange.RequiredBytes, req.Parameters)
	if err != nil {
		return fmt.Errorf("configure fs: %w", err)
	}

	devices := df.FsMeta.RequestBlockDevices()
	for k, v := range devices {
		prefix := "block." + k
		params := df.p
		err = serialmaps.Parse(req.Parameters, prefix, &params, serialmaps.WithRemove(true))
		if err != nil {
			return fmt.Errorf("parse params for '%s': %w", prefix, err)
		}
		df.Block[k] = BlockInfo{
			Req:         v,
			p:           params,
			publishData: map[string]json.RawMessage{},
		}
	}

	return nil
}

func (df *DispatchBlockFs) InitHandles() error {
	df.srv = df.conf.FindServerByName(df.p.ServerName)
	if df.srv == nil {
		return fmt.Errorf("server not found: '%s'", df.p.ServerName)
	}

	for k, v := range df.Block {
		srv := df.conf.FindServerByName(v.p.ServerName)
		if srv == nil {
			return fmt.Errorf("block '%s': server not found: '%s'", k, v.p.ServerName)
		}
		tr := srv.trInstance

		for name := range strings.SplitSeq(v.p.AddressNames, ",") {
			found := false
			for _, a := range srv.Conf.AddressList {
				if a.Name == name {
					v.addrOpt = append(v.addrOpt, a)
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("address '%s' is not found", name)
			}
		}

		str, err := blkvolfactory.StorageFromType(tr, v.p.StorageType, srv.Conf.GlobalParams)
		if err != nil {
			return fmt.Errorf("block '%s': find storage: %w", k, err)
		}
		v.storageManager = str

		shr, err := blkshrfactory.ShareFromType(tr, v.p.ShareType, srv.Conf.GlobalParams)
		if err != nil {
			return fmt.Errorf("block '%s': find share: %w", k, err)
		}
		v.shareManager = shr

		strCaps := str.GetCapabilities()
		shrCaps := shr.GetCapabilities()

		if shrCaps.OpaqueVolume != "" && shrCaps.OpaqueVolume != strCaps.OpaqueType {
			return fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", df.p.ShareType, shrCaps.OpaqueVolume, df.p.StorageType, strCaps.OpaqueType)
		}

		storageId := df.rawName + "-" + k
		if v.p.StorageIdTemplate != "" {
			storageId, err = generateId("storageIdTemplate", v.p.StorageIdTemplate, df.rawName, df.rawStaticParams)
			if err != nil {
				return fmt.Errorf("block '%s': generate storage id: %w", k, err)
			}
		}

		vol, err := str.CreateVolume(blockvolume.CreateRequest{
			Id:            storageId,
			StaticParams:  df.rawStaticParams,
			MutableParams: df.rawMutableParams,
		})
		if err != nil {
			return fmt.Errorf("block '%s': CreateVolume: %w", k, err)
		}
		v.storageHandle = vol

		shareId := df.rawName + "-" + k
		if v.p.ShareIdTemplate != "" {
			id, err := generateId("shareIdTemplate", v.p.ShareIdTemplate, df.rawName, df.rawStaticParams)
			if err != nil {
				return fmt.Errorf("generate share id: %w", err)
			}
			shareId = id
		}

		shareHandle, err := shr.CreateShare(blockshare.CreateShareRequest{
			ShareId:       shareId,
			StaticParams:  df.rawStaticParams,
			MutableParams: df.rawMutableParams,
		})
		if err != nil {
			return fmt.Errorf("block '%s': CreateShare: %w", k, err)
		}
		v.shareHandle = shareHandle

		df.Block[k] = v
	}

	return nil
}

func (df *DispatchBlockFs) Create(ctx context.Context) error {
	dbEntry, err := df.srv.dbInstance.CreateEntry(ctx, df.rawName, df.uuid)
	if err != nil {
		return fmt.Errorf("create db entry: %w", err)
	}
	df.dbEntry = dbEntry

	props := map[string]filecon.StorageProperties{}
	for k, v := range df.Block {
		strCaps := v.storageManager.GetCapabilities()
		err := v.storageHandle.CreateState(ctx, v.Req.MinSize)
		if err != nil {
			return fmt.Errorf("block '%s': storageHandle.CreateState: %w", k, err)
		}
		err = df.Save(ctx)
		if err != nil {
			return fmt.Errorf("save after storageHandle.CreateState: %w", err)
		}
		state, err := v.storageHandle.ReadState(ctx)
		if err != nil {
			return fmt.Errorf("block '%s': storageHandle.ReadState: %w", k, err)
		}
		props[k] = filecon.StorageProperties{
			Size:             state.Size,
			InitialEmpty:     strCaps.BlockInitialEmpty,
			RMWSize:          state.RMWSize,
			OptimalWriteSize: state.OptimalWriteSize,
			AtomicSize:       state.AtomicSize,
		}
		err = v.shareHandle.PrepareShare(ctx)
		if err != nil {
			return fmt.Errorf("block '%s': shareHandle.PrepareShare: %w", k, err)
		}
		shrCaps := v.shareManager.GetCapabilities()
		if shrCaps.OpaqueVolume != "" {
			err = v.shareHandle.AttachVolume(ctx, v.storageHandle, 0)
		} else {
			err = v.shareHandle.AttachPath(ctx, state.FsPath, 0)
		}
		if err != nil {
			return fmt.Errorf("block '%s': shareHandle.AttachPath: %w", k, err)
		}
		err = df.Save(ctx)
		if err != nil {
			return fmt.Errorf("save after shareHandle.AttachPath: %w", err)
		}
	}

	err = df.FsMeta.SetVolumeProps(props)
	if err != nil {
		return fmt.Errorf("FsMeta.SetVolumeProps: %w", err)
	}
	err = df.Save(ctx)
	if err != nil {
		return fmt.Errorf("save after FsMeta.SetVolumeProps: %w", err)
	}

	return nil
}

func (df *DispatchBlockFs) GetVolumeContext() volumecontext.VolumeContext {
	return volumecontext.VolumeContext{
		Uuid:        df.uuid,
		ServerName:  df.p.ServerName,
		StorageType: df.p.StorageType,
		ShareType:   df.p.ShareType,
		FsType:      df.FsType,
		// TODO customize
		FsRootDirMode: "2777",
	}
}

func (df *DispatchBlockFs) GetSize() int64 {
	return df.FsMeta.GetSize()
}

func (df *DispatchBlockFs) SetSize(ctx context.Context, newSize int64) error {
	err := df.FsMeta.Configure(newSize, df.rawStaticParams)
	if err != nil {
		return fmt.Errorf("configure fs: %w", err)
	}

	devices := df.FsMeta.RequestBlockDevices()
	for k, v := range devices {
		block, ok := df.Block[k]
		if !ok {
			return fmt.Errorf("fs requested block '%s' which was not present before resize", k)
		}
		err = block.storageHandle.SetSize(ctx, v.MinSize)
		if err != nil {
			return fmt.Errorf("block '%s': resize: %w", k, err)
		}
		err = df.Save(ctx)
		if err != nil {
			return fmt.Errorf("save after storageHandle.SetSize: %w", err)
		}
	}
	return nil
}

func (df *DispatchBlockFs) Publish(ctx context.Context, nodeName string) (publishcontext.PublishContext, error) {
	req := blockshare.PublishRequest{
		NodeName: nodeName,
	}
	result := publishcontext.PublishContext{
		BlockDevices: map[string]publishcontext.Block{},
		Filesystem:   df.FsMeta.GetData(),
	}
	for k, v := range df.Block {
		var addrOpt []string
		for _, a := range v.addrOpt {
			addrOpt = append(addrOpt, a.Address)
		}
		req.AddressList = addrOpt
		logs.FromCtx(ctx).MicroOp("publish block", "name", k, "request", req)
		sharePc, err := v.shareHandle.Publish(ctx, req)
		if err != nil {
			return result, fmt.Errorf("block '%s': shareHandle.Publish: %w", k, err)
		}
		logs.FromCtx(ctx).MicroOp("publish block (result)", "name", k, "result", sharePc)
		result.BlockDevices[k] = publishcontext.Block{
			ConnectorId:     v.p.ConnectorId,
			Data:            sharePc.Protocol,
			ConnectorPreset: v.p.BlockPreset,
			CgroupPreset:    v.p.CgroupPreset,
		}
		if sharePc.LocalData != nil {
			serializedData, err := json.Marshal(sharePc.LocalData)
			if err != nil {
				return result, fmt.Errorf("block '%s': serialize local data: %w", k, err)
			}
			v.publishData[req.NodeName] = serializedData
			err = df.Save(ctx)
			if err != nil {
				return result, fmt.Errorf("save after shareHandle.Publish: %w", err)
			}
		}
	}
	return result, nil
}

func (df *DispatchBlockFs) Unpublish(ctx context.Context, nodeId string) error {
	for k, v := range df.Block {
		data, ok := v.publishData[nodeId]
		if !ok || data == nil {
			continue
		}
		err := v.shareHandle.Unpublish(ctx, data)
		if err != nil {
			return fmt.Errorf("block '%s': shareHandle.Unpublish: %w", k, err)
		}
		err = df.Save(ctx)
		if err != nil {
			return fmt.Errorf("save: %w", err)
		}
	}
	return nil
}

func (df *DispatchBlockFs) UnpublishAll(ctx context.Context) error {
	for k, v := range df.Block {
		caps := v.shareManager.GetCapabilities()
		if caps.UnpublishAll {
			err := v.shareHandle.UnpublishAll(ctx)
			if err != nil {
				return fmt.Errorf("block '%s': shareHandle.UnpublishAll: %w", k, err)
			}
			err = df.Save(ctx)
			if err != nil {
				return fmt.Errorf("save: %w", err)
			}
			continue
		}
		for _, pd := range v.publishData {
			if pd == nil {
				continue
			}
			err := v.shareHandle.Unpublish(ctx, pd)
			if err != nil {
				return fmt.Errorf("block '%s': shareHandle.Unpublish: %w", k, err)
			}
			err = df.Save(ctx)
			if err != nil {
				return fmt.Errorf("save: %w", err)
			}
		}
	}
	return nil
}

func (df *DispatchBlockFs) Destroy(ctx context.Context) error {
	err := df.UnpublishAll(ctx)
	if err != nil {
		return fmt.Errorf("unpublishAll")
	}
	for k, v := range df.Block {
		if v.shareHandle != nil {
			err := v.shareHandle.DeleteShare(ctx)
			if err != nil {
				return fmt.Errorf("block '%s': shareHandle.DeleteShare: %w", k, err)
			}
			v.shareHandle = nil
			err = df.Save(ctx)
			if err != nil {
				return fmt.Errorf("save: %w", err)
			}
		}
		if v.storageHandle != nil {
			err := v.storageHandle.DeleteState(ctx)
			if err != nil {
				return fmt.Errorf("block '%s': storageHandle.DeleteState: %w", k, err)
			}
			v.storageHandle = nil
			err = df.Save(ctx)
			if err != nil {
				return fmt.Errorf("save: %w", err)
			}
		}
		df.Block[k] = v
	}
	return nil
}

type SerializedDispatchBlockFs struct {
	RawName string `json:"rawName"`
	Uuid    string `json:"uuid"`

	RawStaticParams  map[string]string `json:"rawStaticParams"`
	RawMutableParams map[string]string `json:"rawMutableParams"`

	FsType string          `json:"fsType"`
	FsMeta json.RawMessage `json:"fsMeta"`

	P DispatchParams `json:"p"`

	Block map[string]SerializedBlockInfo `json:"block"`
}

type SerializedBlockInfo struct {
	Req         filecon.BlockDeviceRequirement `json:"req"`
	StorageData json.RawMessage                `json:"storageData"`
	ShareData   json.RawMessage                `json:"shareData"`
	P           DispatchParams                 `json:"p"`
	PublishData map[string]json.RawMessage     `json:"publishData"`
	AddrOpt     []string                       `json:"addrOpt"`
}

func (df *DispatchBlockFs) Save(ctx context.Context) error {
	result := SerializedDispatchBlockFs{
		RawName:          df.rawName,
		Uuid:             df.uuid,
		RawStaticParams:  df.rawStaticParams,
		RawMutableParams: df.rawMutableParams,
		FsType:           df.FsType,
		P:                df.p,
		Block:            map[string]SerializedBlockInfo{},
	}
	fsMetaSerialized, err := df.FsMeta.Serialize()
	if err != nil {
		return err
	}
	result.FsMeta = fsMetaSerialized

	for k, v := range df.Block {
		storageData, err := v.storageHandle.Serialize()
		if err != nil {
			return fmt.Errorf("block '%s': serialize storage: %w", k, err)
		}
		shareData, err := v.shareHandle.Serialize()
		if err != nil {
			return fmt.Errorf("block '%s': serialize share: %w", k, err)
		}
		var addrOpt []string
		for _, a := range v.addrOpt {
			addrOpt = append(addrOpt, a.StableId)
		}
		result.Block[k] = SerializedBlockInfo{
			Req:         v.Req,
			P:           v.p,
			StorageData: storageData,
			ShareData:   json.RawMessage(shareData),
			PublishData: v.publishData,
			AddrOpt:     addrOpt,
		}
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	w := voldbwrapper.New(df.dbEntry)
	err = w.SetVolumeInfo(ctx, voldbwrapper.VolumeInfo{
		VolumeType: "block-fs",
		Data:       data,
	})
	if err != nil {
		return fmt.Errorf("SetVolumeInfo: %w", err)
	}

	return nil
}

func (ds *SerializedDispatchBlockFs) parse(ctx context.Context, conf *DispatcherConf) (DispatchBlockFs, error) {
	result := DispatchBlockFs{
		conf:             conf,
		rawName:          ds.RawName,
		uuid:             ds.Uuid,
		rawStaticParams:  ds.RawStaticParams,
		rawMutableParams: ds.RawMutableParams,
		FsType:           ds.FsType,
		p:                ds.P,
		Block:            map[string]BlockInfo{},
	}

	result.srv = conf.FindServerByName(ds.P.ServerName)
	if result.srv == nil {
		return result, fmt.Errorf("server not found: '%s'", ds.P.ServerName)
	}

	dbEntry, err := result.srv.dbInstance.CreateEntry(ctx, ds.RawName, ds.Uuid)
	if err != nil {
		return result, fmt.Errorf("find db entry: %w", err)
	}
	result.dbEntry = dbEntry

	fsInfo, err := fsfactory.GetByType(result.FsType)
	if err != nil {
		return result, fmt.Errorf("fs: %w", err)
	}
	if fsInfo.Meta == nil {
		return result, fmt.Errorf("fs: no metadata")
	}
	result.FsMeta = fsInfo.Meta()
	if result.FsMeta == nil {
		return result, fmt.Errorf("fs: nil metadata")
	}
	err = result.FsMeta.Restore(ds.FsMeta)
	if err != nil {
		return result, fmt.Errorf("fs: restore: %w", err)
	}

	for k, v := range ds.Block {
		srv := conf.FindServerByName(v.P.ServerName)
		if srv == nil {
			return result, fmt.Errorf("block '%s': server is missing: '%s'", k, v.P.ServerName)
		}
		tr := srv.trInstance

		str, err := blkvolfactory.StorageFromType(tr, v.P.StorageType, srv.Conf.GlobalParams)
		if err != nil {
			return result, fmt.Errorf("block '%s': find storage: %w", k, err)
		}

		vol, err := str.ParseVolume(v.StorageData)
		if err != nil {
			return result, fmt.Errorf("block '%s': ParseVolume: %w", k, err)
		}

		shr, err := blkshrfactory.ShareFromType(tr, v.P.ShareType, srv.Conf.GlobalParams)
		if err != nil {
			return result, fmt.Errorf("block '%s': find share: %w", k, err)
		}

		shareHandle, err := shr.ParseShare(v.ShareData)
		if err != nil {
			return result, fmt.Errorf("block '%s': ParseShare: %w", k, err)
		}

		var addrOpt []AddressOption
		for _, a := range v.AddrOpt {
			found := false
			for _, v := range srv.Conf.AddressList {
				if v.StableId == a {
					addrOpt = append(addrOpt, v)
					found = true
					break
				}
			}
			if !found {
				return result, fmt.Errorf("address with id '%s' is not found", a)
			}
		}

		if v.PublishData == nil {
			v.PublishData = map[string]json.RawMessage{}
		}

		result.Block[k] = BlockInfo{
			storageManager: str,
			storageHandle:  vol,
			shareManager:   shr,
			shareHandle:    shareHandle,
			p:              v.P,
			Req:            v.Req,
			publishData:    v.PublishData,
			addrOpt:        addrOpt,
		}
	}

	return result, nil
}

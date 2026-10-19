package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/api/publishcontext"
	"meoe.io/poly-csi/pkg/api/volumecontext"
	"meoe.io/poly-csi/pkg/controller/fileshare"
	"meoe.io/poly-csi/pkg/controller/fileshare/filesharefactory"
	"meoe.io/poly-csi/pkg/controller/filevol"
	"meoe.io/poly-csi/pkg/controller/filevol/filevolfactory"
	"meoe.io/poly-csi/pkg/controller/voldb"
	"meoe.io/poly-csi/pkg/controller/voldbwrapper"
)

type DispatchNativeFs struct {
	conf *DispatcherConf

	srv *parsedServer

	rawName string
	uuid    string
	size    int64

	rawStaticParams  map[string]string
	rawMutableParams map[string]string

	dbEntry voldb.Entry

	FsType string

	p DispatchParams

	str filevol.Manager
	vol filevol.Volume

	shareManager fileshare.Manager
	shareHandle  fileshare.Share
	addrOpt      []AddressOption

	publishData map[string]json.RawMessage
}

var _ VolumeDriver = &DispatchNativeFs{}

func NewDispatchNativeFs(conf *DispatcherConf, uuid string) VolumeDriver {
	return &DispatchNativeFs{
		conf: conf,
		uuid: uuid,
	}
}

func (df *DispatchNativeFs) MakeVolumeId() string {
	return createVolumeId(df.srv.Conf.StableId, df.rawName)
}

func (df *DispatchNativeFs) ParseRequest(req *csi.CreateVolumeRequest) error {
	df.rawName = req.Name

	fsCaps := req.VolumeCapabilities[0].GetMount()
	if fsCaps.FsType == "" {
		return fmt.Errorf("missing fs type")
	}
	df.FsType = fsCaps.FsType
	df.size = req.CapacityRange.RequiredBytes

	df.rawStaticParams = req.Parameters
	df.rawMutableParams = req.MutableParameters

	dispatchParams, err := parseDispatchParams(paramsPrefix, nil, req.Parameters, req.MutableParameters)
	if err != nil {
		return fmt.Errorf("parseDispatchParams: %w", err)
	}
	df.p = dispatchParams

	return nil
}

func (df *DispatchNativeFs) InitHandles() error {
	df.srv = df.conf.FindServerByName(df.p.ServerName)
	if df.srv == nil {
		return fmt.Errorf("server not found: '%s'", df.p.ServerName)
	}

	srv := df.conf.FindServerByName(df.p.ServerName)
	if srv == nil {
		return fmt.Errorf("server is missing: '%s'", df.p.ServerName)
	}
	tr := srv.trInstance

	df.addrOpt = nil
	for name := range strings.SplitSeq(df.p.AddressNames, ",") {
		found := false
		for _, v := range srv.Conf.AddressList {
			if v.Name == name {
				df.addrOpt = append(df.addrOpt, v)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("address '%s' is not found", name)
		}
	}

	str, err := filevolfactory.StorageFromType(tr, df.p.StorageType, srv.Conf.GlobalParams)
	if err != nil {
		return fmt.Errorf("find storage: %w", err)
	}
	df.str = str

	shr, err := filesharefactory.ShareFromType(tr, df.p.ShareType, srv.Conf.GlobalParams)
	if err != nil {
		return fmt.Errorf("find share: %w", err)
	}
	df.shareManager = shr

	strCaps := str.GetCapabilities()
	shrCaps := shr.GetCapabilities()

	if shrCaps.OpaqueVolume != "" && shrCaps.OpaqueVolume != strCaps.OpaqueType {
		return fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", df.p.ShareType, shrCaps.OpaqueVolume, df.p.StorageType, strCaps.OpaqueType)
	}

	storageId := df.rawName
	if df.p.StorageIdTemplate != "" {
		storageId, err = generateId("storageIdTemplate", df.p.StorageIdTemplate, df.rawName, df.rawStaticParams)
		if err != nil {
			return fmt.Errorf("generate storage id: %w", err)
		}
	}

	vol, err := str.CreateVolume(filevol.CreateRequest{
		Id:            storageId,
		StaticParams:  df.rawStaticParams,
		MutableParams: df.rawMutableParams,
	})
	if err != nil {
		return fmt.Errorf("CreateVolume: %w", err)
	}

	df.vol = vol

	shareId := df.rawName
	if df.p.ShareIdTemplate != "" {
		id, err := generateId("shareIdTemplate", df.p.ShareIdTemplate, df.rawName, df.rawStaticParams)
		if err != nil {
			return fmt.Errorf("generate share id: %w", err)
		}
		shareId = id
	}

	shareHandle, err := shr.CreateShare(fileshare.CreateShareRequest{
		ShareId:       shareId,
		StaticParams:  df.rawStaticParams,
		MutableParams: df.rawMutableParams,
	})
	if err != nil {
		return fmt.Errorf("CreateShare: %w", err)
	}
	df.shareHandle = shareHandle

	return nil
}

func (df *DispatchNativeFs) Create(ctx context.Context) error {
	dbEntry, err := df.srv.dbInstance.CreateEntry(ctx, df.rawName, df.uuid)
	if err != nil {
		return fmt.Errorf("create db entry: %w", err)
	}
	df.dbEntry = dbEntry

	err = df.vol.CreateState(ctx, df.size)
	if err != nil {
		return fmt.Errorf("storageHandle.CreateState: %w", err)
	}
	err = df.Save(ctx)
	if err != nil {
		return fmt.Errorf("save after storageHandle.CreateState: %w", err)
	}
	state, err := df.vol.ReadState(ctx)
	if err != nil {
		return fmt.Errorf("storageHandle.ReadState: %w", err)
	}
	err = df.shareHandle.PrepareShare(ctx)
	if err != nil {
		return fmt.Errorf("shareHandle.PrepareShare: %w", err)
	}
	err = df.Save(ctx)
	if err != nil {
		return fmt.Errorf("save after shareHandle.PrepareShare: %w", err)
	}
	shrCaps := df.shareManager.GetCapabilities()
	if shrCaps.OpaqueVolume != "" {
		err = df.shareHandle.AttachVolume(ctx, df.vol)
	} else {
		err = df.shareHandle.AttachPath(ctx, state.FsPath)
	}
	if err != nil {
		return fmt.Errorf("shareHandle.AttachPath: %w", err)
	}
	err = df.Save(ctx)
	if err != nil {
		return fmt.Errorf("save after shareHandle.AttachPath: %w", err)
	}

	return nil
}

func (df *DispatchNativeFs) GetVolumeContext() volumecontext.VolumeContext {
	var presets []string
	if df.p.NodePresets != "" {
		for v := range strings.SplitSeq(df.p.NodePresets, ",") {
			presets = append(presets, v)
		}
	}

	return volumecontext.VolumeContext{
		Uuid:        df.uuid,
		ServerName:  df.p.ServerName,
		StorageType: df.p.StorageType,
		ShareType:   df.p.ShareType,
		Presets:     presets,
		FsType:      df.FsType,
		// TODO customize
		FsRootDirMode: "2777",
	}
}

func (df *DispatchNativeFs) GetSize() int64 {
	return df.size
}

func (df *DispatchNativeFs) SetSize(ctx context.Context, newSize int64) error {
	return df.vol.SetSize(ctx, newSize)
}

func (df *DispatchNativeFs) Publish(ctx context.Context, nodeName string) (publishcontext.PublishContext, error) {
	result := publishcontext.PublishContext{
		BlockDevices: nil,
	}

	var addrOpt []string
	for _, a := range df.addrOpt {
		addrOpt = append(addrOpt, a.Address)
	}
	req := fileshare.PublishRequest{
		NodeName:    nodeName,
		AddressList: addrOpt,
	}
	sharePc, err := df.shareHandle.Publish(ctx, req)
	if err != nil {
		return result, fmt.Errorf("shareHandle.Publish: %w", err)
	}
	result.Filesystem = sharePc.Protocol
	if sharePc.LocalData != nil {
		serializedData, err := json.Marshal(sharePc.LocalData)
		if err != nil {
			return result, fmt.Errorf("serialize local data: %w", err)
		}
		df.publishData[req.NodeName] = serializedData
	}
	return result, nil
}

func (df *DispatchNativeFs) Unpublish(ctx context.Context, nodeId string) error {
	data, ok := df.publishData[nodeId]
	if !ok || data == nil {
		return nil
	}
	err := df.shareHandle.Unpublish(ctx, data)
	if err != nil {
		return fmt.Errorf("shareHandle.Unpublish: %w", err)
	}
	err = df.Save(ctx)
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func (df *DispatchNativeFs) UnpublishAll(ctx context.Context) error {
	caps := df.shareManager.GetCapabilities()
	if caps.UnpublishAll {
		err := df.shareHandle.UnpublishAll(ctx)
		if err != nil {
			return fmt.Errorf("shareHandle.UnpublishAll: %w", err)
		}
		err = df.Save(ctx)
		if err != nil {
			return fmt.Errorf("save: %w", err)
		}
		return nil
	}
	for _, pd := range df.publishData {
		if pd == nil {
			continue
		}
		err := df.shareHandle.Unpublish(ctx, pd)
		if err != nil {
			return fmt.Errorf("shareHandle.Unpublish: %w", err)
		}
		err = df.Save(ctx)
		if err != nil {
			return fmt.Errorf("save: %w", err)
		}
	}
	return nil
}

func (df *DispatchNativeFs) Destroy(ctx context.Context) error {
	if df.shareHandle != nil {
		err := df.shareHandle.DeleteShare(ctx)
		if err != nil {
			return fmt.Errorf("shareHandle.DeleteShare: %w", err)
		}
		df.shareHandle = nil
	}
	if df.vol != nil {
		err := df.vol.DeleteState(ctx)
		if err != nil {
			return fmt.Errorf("storageHandle.DeleteState: %w", err)
		}
		df.vol = nil
	}
	return nil
}

type SerializedDispatchNativeFs struct {
	RawName string `json:"rawName"`
	Uuid    string `json:"uuid"`
	Size    int64  `json:"size"`

	RawStaticParams  map[string]string `json:"rawStaticParams"`
	RawMutableParams map[string]string `json:"rawMutableParams"`

	P DispatchParams `json:"p"`

	StorageData json.RawMessage            `json:"storageData"`
	ShareData   json.RawMessage            `json:"shareData"`
	PublishData map[string]json.RawMessage `json:"publishData"`
	AddrOpt     []string                   `json:"addrOpt"`
}

func (df *DispatchNativeFs) Save(ctx context.Context) error {
	storageData, err := df.vol.Serialize()
	if err != nil {
		return fmt.Errorf("serialize storage: %w", err)
	}
	shareData, err := df.shareHandle.Serialize()
	if err != nil {
		return fmt.Errorf("serialize share: %w", err)
	}

	var addrOpt []string
	for _, a := range df.addrOpt {
		addrOpt = append(addrOpt, a.StableId)
	}

	result := SerializedDispatchNativeFs{
		RawName:          df.rawName,
		Uuid:             df.uuid,
		Size:             df.size,
		RawStaticParams:  df.rawStaticParams,
		RawMutableParams: df.rawMutableParams,
		P:                df.p,
		StorageData:      storageData,
		ShareData:        shareData,
		PublishData:      df.publishData,
		AddrOpt:          addrOpt,
	}
	_ = result

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	w := voldbwrapper.New(df.dbEntry)
	err = w.SetVolumeInfo(ctx, voldbwrapper.VolumeInfo{
		VolumeType: "native-fs",
		Data:       data,
	})
	if err != nil {
		return fmt.Errorf("SetVolumeInfo: %w", err)
	}

	return nil
}

func (ds *SerializedDispatchNativeFs) parse(ctx context.Context, conf *DispatcherConf) (DispatchNativeFs, error) {
	result := DispatchNativeFs{
		conf:             conf,
		rawName:          ds.RawName,
		uuid:             ds.Uuid,
		size:             ds.Size,
		rawStaticParams:  ds.RawStaticParams,
		rawMutableParams: ds.RawMutableParams,
		p:                ds.P,
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

	tr := result.srv.trInstance

	str, err := filevolfactory.StorageFromType(tr, ds.P.StorageType, result.srv.Conf.GlobalParams)
	if err != nil {
		return result, fmt.Errorf("find storage: %w", err)
	}
	result.str = str

	vol, err := str.ParseVolume(ds.StorageData)
	if err != nil {
		return result, fmt.Errorf("ParseVolume: %w", err)
	}
	result.vol = vol

	shr, err := filesharefactory.ShareFromType(tr, ds.P.ShareType, result.srv.Conf.GlobalParams)
	if err != nil {
		return result, fmt.Errorf("find share: %w", err)
	}
	result.shareManager = shr

	shareHandle, err := shr.ParseShare(ds.ShareData)
	if err != nil {
		return result, fmt.Errorf("ParseShare: %w", err)
	}
	result.shareHandle = shareHandle

	for _, a := range ds.AddrOpt {
		found := false
		for _, v := range result.srv.Conf.AddressList {
			if v.StableId == a {
				result.addrOpt = append(result.addrOpt, v)
				found = true
				break
			}
		}
		if !found {
			return result, fmt.Errorf("address with id '%s' is not found", a)
		}
	}

	return result, nil
}

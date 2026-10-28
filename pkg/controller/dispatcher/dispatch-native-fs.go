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
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type DispatchNativeFs struct {
	conf *DispatcherConf

	dbSrv *parsedServer
	fsSrv *parsedServer

	rawName string
	uuid    string
	size    int64

	idHelper IdHelper
	parser   *serialmaps.Parser

	dbEntry voldb.Entry

	serverName string
	fsParams   DispatchFsParams

	str filevol.Manager
	vol filevol.Volume

	shareManager fileshare.Manager
	shareHandle  fileshare.Share
	addrOpt      []AddressOption

	publishData map[string]json.RawMessage
}

type DispatchFsParams struct {
	ServerName        string `map:"server" validate:"required" json:"serverName"`
	ServerId          string `map:"-" json:"serverId"`
	StorageIdTemplate string `map:"storage.id" json:"storageIdTemplate"`
	StorageType       string `map:"storage.type" validate:"required" json:"storageType"`
	ShareType         string `map:"share.type" json:"shareType"`
	ShareIdTemplate   string `map:"share.id" json:"shareIdTemplate"`
	AddressNames      string `map:"address" validate:"required" json:"addressNames"`
	ConnectorId       string `map:"connector.id" validate:"required" json:"connectorId"`
	BlockPreset       string `map:"connector.preset" json:"blockPreset"`
	CgroupPreset      string `map:"cgroupPreset" json:"cgroupPreset"`
}

var _ VolumeDriver = &DispatchNativeFs{}

func NewDispatchNativeFs(conf *DispatcherConf, uuid string) VolumeDriver {
	return &DispatchNativeFs{
		conf: conf,
		uuid: uuid,
	}
}

func (df *DispatchNativeFs) MakeVolumeId() string {
	return createVolumeId(df.dbSrv.Conf.StableId, df.rawName)
}

func (df *DispatchNativeFs) ParseRequest(req *csi.CreateVolumeRequest, parser *serialmaps.Parser) error {
	df.rawName = req.Name
	df.parser = parser
	df.idHelper = NewIdHelper(req.Name, req.Parameters)
	df.serverName = parser.WithPrefix(dispatchPrefix).Take("server")

	df.size = req.CapacityRange.RequiredBytes

	err := parser.WithPrefix("fs").Parse(&df.fsParams)
	if err != nil {
		return fmt.Errorf("parse fs params: %w", err)
	}

	if df.fsParams.ServerName == "" {
		df.fsParams.ServerName = df.serverName
	}

	return nil
}

func (df *DispatchNativeFs) InitHandles() error {
	df.dbSrv = df.conf.FindServerByName(df.serverName)
	if df.dbSrv == nil {
		return fmt.Errorf("server not found: '%s'", df.serverName)
	}
	df.fsSrv = df.conf.FindServerByName(df.fsParams.ServerName)
	if df.fsSrv == nil {
		return fmt.Errorf("server not found: '%s'", df.fsParams.ServerName)
	}

	srv := df.fsSrv
	tr := srv.trInstance

	df.fsParams.ServerId = srv.Conf.StableId

	df.addrOpt = nil
	for name := range strings.SplitSeq(df.fsParams.AddressNames, ",") {
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

	str, err := filevolfactory.StorageFromType(tr, df.fsParams.StorageType, srv.Conf.GlobalParams)
	if err != nil {
		return fmt.Errorf("find storage: %w", err)
	}
	df.str = str

	shr, err := filesharefactory.ShareFromType(tr, df.fsParams.ShareType, srv.Conf.GlobalParams)
	if err != nil {
		return fmt.Errorf("find share: %w", err)
	}
	df.shareManager = shr

	strCaps := str.GetCapabilities()
	shrCaps := shr.GetCapabilities()

	if shrCaps.OpaqueVolume != "" && shrCaps.OpaqueVolume != strCaps.OpaqueType {
		return fmt.Errorf("share %s accepts volumes %v but storage %s provides %s", df.fsParams.ShareType, shrCaps.OpaqueVolume, df.fsParams.StorageType, strCaps.OpaqueType)
	}

	storageId := df.rawName
	if df.fsParams.StorageIdTemplate != "" {
		storageId, err = df.idHelper.Generate(df.fsParams.StorageIdTemplate)
		if err != nil {
			return fmt.Errorf("generate storage id: %w", err)
		}
	}

	vol, err := str.CreateVolume(filevol.CreateRequest{
		Id:     storageId,
		Parser: df.parser.WithPrefix("fs.storage"),
	})
	if err != nil {
		return fmt.Errorf("CreateVolume: %w", err)
	}

	df.vol = vol

	shareId := df.rawName
	if df.fsParams.ShareIdTemplate != "" {
		id, err := df.idHelper.Generate(df.fsParams.ShareIdTemplate)
		if err != nil {
			return fmt.Errorf("generate share id: %w", err)
		}
		shareId = id
	}

	shareHandle, err := shr.CreateShare(fileshare.CreateShareRequest{
		ShareId: shareId,
		Parser:  df.parser.WithPrefix("fs.share"),
	})
	if err != nil {
		return fmt.Errorf("CreateShare: %w", err)
	}
	df.shareHandle = shareHandle

	return nil
}

func (df *DispatchNativeFs) Create(ctx context.Context) error {
	dbEntry, err := df.dbSrv.dbInstance.CreateEntry(ctx, df.rawName, df.uuid)
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
	return volumecontext.VolumeContext{
		Uuid:        df.uuid,
		ServerName:  df.fsParams.ServerName,
		StorageType: df.fsParams.StorageType,
		ShareType:   df.fsParams.ShareType,
		FsType:      df.fsParams.ConnectorId,
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

	ServerName string           `json:"serverName"`
	FsParams   DispatchFsParams `json:"fsParams"`

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
		RawName:     df.rawName,
		Uuid:        df.uuid,
		Size:        df.size,
		ServerName:  df.serverName,
		FsParams:    df.fsParams,
		StorageData: storageData,
		ShareData:   shareData,
		PublishData: df.publishData,
		AddrOpt:     addrOpt,
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

func (ds *SerializedDispatchNativeFs) parse(ctx context.Context, conf *DispatcherConf, dbSrv *parsedServer) (DispatchNativeFs, error) {
	result := DispatchNativeFs{
		conf:       conf,
		dbSrv:      dbSrv,
		rawName:    ds.RawName,
		uuid:       ds.Uuid,
		size:       ds.Size,
		serverName: ds.ServerName,
		fsParams:   ds.FsParams,
	}

	result.fsSrv = conf.FindServerById(ds.FsParams.ServerId)
	if result.fsSrv == nil {
		return result, fmt.Errorf("server not found: 'id=%s'", ds.FsParams.ServerId)
	}

	dbEntry, err := result.dbSrv.dbInstance.CreateEntry(ctx, ds.RawName, ds.Uuid)
	if err != nil {
		return result, fmt.Errorf("find db entry: %w", err)
	}
	result.dbEntry = dbEntry

	srv := result.fsSrv
	tr := srv.trInstance

	str, err := filevolfactory.StorageFromType(tr, ds.FsParams.StorageType, srv.Conf.GlobalParams)
	if err != nil {
		return result, fmt.Errorf("find storage: %w", err)
	}
	result.str = str

	vol, err := str.ParseVolume(ds.StorageData)
	if err != nil {
		return result, fmt.Errorf("ParseVolume: %w", err)
	}
	result.vol = vol

	shr, err := filesharefactory.ShareFromType(tr, ds.FsParams.ShareType, srv.Conf.GlobalParams)
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
		for _, v := range srv.Conf.AddressList {
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

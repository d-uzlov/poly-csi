package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/api/publishcontext"
	"meoe.io/poly-csi/pkg/controller/fileshare"
	"meoe.io/poly-csi/pkg/controller/fileshare/filesharefactory"
	"meoe.io/poly-csi/pkg/controller/filevol"
	"meoe.io/poly-csi/pkg/controller/filevol/filevolfactory"
)

type DispatchNativeFs struct {
	conf *DispatcherConf

	srv *parsedServer

	rawName string
	uuid    string
	size    int64

	rawStaticParams  map[string]string
	rawMutableParams map[string]string

	FsType string

	p DispatchParams

	str filevol.Manager
	vol filevol.Volume

	shareManager fileshare.Manager
	shareHandle  fileshare.Share
	addrOpt      []AddressOption

	publishData map[string]json.RawMessage
}

func NewDispatchNativeFs(conf *DispatcherConf, uuid string) DispatchNativeFs {
	return DispatchNativeFs{
		conf: conf,
		uuid: uuid,
	}
}

func (df *DispatchNativeFs) parseRequest(req *csi.CreateVolumeRequest) error {
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

func (df *DispatchNativeFs) initHandles() error {
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

	shr, err := filesharefactory.ShareFromType(tr, df.p.StorageType, srv.Conf.GlobalParams)
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

func (df *DispatchNativeFs) create(ctx context.Context) error {
	err := df.vol.CreateState(ctx, df.size)
	if err != nil {
		return fmt.Errorf("storageHandle.CreateState: %w", err)
	}
	err = df.save(ctx)
	if err != nil {
		return fmt.Errorf("save after storageHandle.CreateState: %w", err)
	}
	state, err := df.vol.ReadState(ctx)
	if err != nil {
		return fmt.Errorf("storageHandle.ReadState: %w", err)
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
	err = df.save(ctx)
	if err != nil {
		return fmt.Errorf("save after shareHandle.AttachPath: %w", err)
	}

	return nil
}

func (df *DispatchNativeFs) publish(ctx context.Context, req fileshare.PublishRequest) (publishcontext.PublishContext, error) {
	result := publishcontext.PublishContext{
		BlockDevices: nil,
	}

	var addrOpt []string
	for _, a := range df.addrOpt {
		addrOpt = append(addrOpt, a.Address)
	}
	req.AddressList = addrOpt
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

func (df *DispatchNativeFs) destroy(ctx context.Context) error {
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

	RawStaticParams  map[string]string `json:"rawStaticParams"`
	RawMutableParams map[string]string `json:"rawMutableParams"`

	P DispatchParams `json:"p"`

	StorageData json.RawMessage            `json:"storageData"`
	ShareData   json.RawMessage            `json:"shareData"`
	PublishData map[string]json.RawMessage `json:"publishData"`
	AddrOpt     []string                   `json:"addrOpt"`
}

func (df *DispatchNativeFs) save(ctx context.Context) error {
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
		RawStaticParams:  df.rawStaticParams,
		RawMutableParams: df.rawMutableParams,
		P:                df.p,
		StorageData:      storageData,
		ShareData:        shareData,
		PublishData:      df.publishData,
		AddrOpt:          addrOpt,
	}
	_ = result

	// TODO write to file

	return nil
}

func (df *SerializedDispatchNativeFs) parse(ctx context.Context, conf *DispatcherConf) error {
	result := DispatchNativeFs{
		conf:             conf,
		rawName:          df.RawName,
		uuid:             df.Uuid,
		rawStaticParams:  df.RawStaticParams,
		rawMutableParams: df.RawMutableParams,
		p:                df.P,
	}
	srv := conf.FindServerByName(df.P.ServerName)
	if srv == nil {
		return fmt.Errorf("server is missing: '%s'", df.P.ServerName)
	}
	tr := srv.trInstance

	str, err := filevolfactory.StorageFromType(tr, df.P.StorageType, srv.Conf.GlobalParams)
	if err != nil {
		return fmt.Errorf("find storage: %w", err)
	}
	result.str = str

	vol, err := str.ParseVolume(df.StorageData)
	if err != nil {
		return fmt.Errorf("ParseVolume: %w", err)
	}
	result.vol = vol

	shr, err := filesharefactory.ShareFromType(tr, df.P.StorageType, srv.Conf.GlobalParams)
	if err != nil {
		return fmt.Errorf("find share: %w", err)
	}
	result.shareManager = shr

	shareHandle, err := shr.ParseShare(df.ShareData)
	if err != nil {
		return fmt.Errorf("ParseShare: %w", err)
	}
	result.shareHandle = shareHandle

	for _, a := range df.AddrOpt {
		found := false
		for _, v := range srv.Conf.AddressList {
			if v.StableId == a {
				result.addrOpt = append(result.addrOpt, v)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("address with id '%s' is not found", a)
		}
	}

	return nil
}

package dispatcher

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/controller/storage/storagefactory"
	"meoe.io/poly-csi/pkg/logs"
)

// ==== misc ====

func (d *Dispatcher) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	dispatchParams, err := parseDispatchParams(nil, req.Parameters, nil)
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

	strInstance, err := storagefactory.StorageFromType(srv.trInstance, dispatchParams.StorageType, srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	available, err := strInstance.GetAvailableCapacity(ctx, req.Parameters)
	if err != nil {
		return nil, fmt.Errorf("server[name=%s]: storage[%s]: GetAvailableCapacity: %w", dispatchParams.ServerName, dispatchParams.StorageType, err)
	}

	return &csi.GetCapacityResponse{
		AvailableCapacity: available,
	}, nil
}

// TODO implement ListVolumes?
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

	strInstance, err := storagefactory.StorageFromType(mh.srv.trInstance, mh.vi.StorageType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	vol, err := strInstance.ParseVolume(mh.vi.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	volumeState, err := vol.ReadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReadState: %w", err)
	}
	if volumeState == nil {
		return nil, fmt.Errorf("ReadState: nil result")
	}

	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      req.VolumeId,
			CapacityBytes: volumeState.Size,
			VolumeContext: mh.vi.VolumeContext,
			// ContentSource: , // TODO restore matching content source
		},
		Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: false,
				Message:  "ok",
			},
		},
	}, nil
}

// TODO implement ControllerModifyVolume
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
	logger := logs.FromCtx(ctx)

	mh, err := d.getVolumeInfo(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	strInstance, err := storagefactory.StorageFromType(mh.srv.trInstance, mh.vi.StorageType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	vol, err := strInstance.ParseVolume(mh.vi.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	newSize := req.CapacityRange.RequiredBytes

	err = vol.Expand(ctx, newSize)
	if err != nil {
		return nil, fmt.Errorf("storage: expand volume: %w", err)
	}
	logger.Info("storage.Expand: success")

	volumeState, err := vol.ReadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReadState: %w", err)
	}
	if volumeState == nil {
		return nil, fmt.Errorf("ReadState: nil result")
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         volumeState.Size,
		NodeExpansionRequired: true,
	}, nil
}

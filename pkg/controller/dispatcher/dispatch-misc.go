package dispatcher

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/storagefactory"
	"meoe.io/poly-csi/pkg/controller/volumecontext"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
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
	mh, err := d.getExistingVolumeHandle(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	strInstance, err := storagefactory.StorageFromType(mh.srv.trInstance, mh.vi.Storage.StorageType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	vol, err := strInstance.ParseVolume(mh.vi.Storage.StorageData)
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

	vcMap, err := serialmaps.Serialize(mh.vi.VolumeContext, volumecontext.Prefix)
	if err != nil {
		return nil, fmt.Errorf("serialize volume context: %w", err)
	}

	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      req.VolumeId,
			CapacityBytes: volumeState.Size,
			VolumeContext: vcMap,
			// ContentSource: , // TODO restore matching content source
		},
		Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
			// PublishedNodeIds: []string{},
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

	mh, err := d.getExistingVolumeHandle(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	strInstance, err := storagefactory.StorageFromType(mh.srv.trInstance, mh.vi.Storage.StorageType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	vol, err := strInstance.ParseVolume(mh.vi.Storage.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	newSize := req.CapacityRange.RequiredBytes

	err = vol.SetSize(ctx, newSize)
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

	mh.vi.Storage.Size = volumeState.Size

	err = mh.wrapper.SetVolumeInfo(ctx, *mh.vi)
	if err != nil {
		return nil, fmt.Errorf("save volume info: %w", err)
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         volumeState.Size,
		NodeExpansionRequired: true,
	}, nil
}

func (d *Dispatcher) ControllerGetVolumeHealth(ctx context.Context, req *csi.ControllerGetVolumeHealthRequest) (*csi.ControllerGetVolumeHealthResponse, error) {
	// logger := logs.FromCtx(ctx)
	result := []*csi.VolumeHealth_VolumeHealthEntry{}

	mh, err := d.getExistingVolumeHandle(ctx, req.VolumeId)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	defer mh.release()
	if mh.vi == nil {
		return nil, fmt.Errorf("volume not found")
	}

	strInstance, err := storagefactory.StorageFromType(mh.srv.trInstance, mh.vi.Storage.StorageType, mh.srv.Conf.GlobalParams)
	if err != nil {
		return nil, err
	}
	vol, err := strInstance.ParseVolume(mh.vi.Storage.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}
	volState, err := vol.ReadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("read volume state: %w", err)
	}

	if mh.vi.Storage.Size != 0 {
		if volState.Size != mh.vi.Storage.Size {
			result = append(result, &csi.VolumeHealth_VolumeHealthEntry{
				Status:  csi.VolumeHealthErrorType_DEGRADED,
				Reason:  "wrong_size",
				Message: fmt.Sprintf("size of the volume changed: initial %d, current %d", mh.vi.Storage.Size, volState.Size),
			})
		}
	}

	if mh.vi.Storage.LocalPathType == storage.VolFolder && mh.vi.Storage.Size != 0 {
		df, err := mh.srv.trInstance.Cmds().Df(ctx, volState.FsPath)
		if err != nil {
			return nil, fmt.Errorf("check free space: %w", err)
		}

		if df.Used+df.Available < mh.vi.Storage.Size {
			result = append(result, &csi.VolumeHealth_VolumeHealthEntry{
				Status:  csi.VolumeHealthErrorType_DEGRADED,
				Reason:  "out_of_space",
				Message: "Volume will not be able to reach capacity due to filesystem running out of space",
			})
		}
	}

	return &csi.ControllerGetVolumeHealthResponse{
		VolumeHealth: &csi.VolumeHealth{
			VolumeId:       req.VolumeId,
			HealthStatuses: result,
		},
	}, nil
}

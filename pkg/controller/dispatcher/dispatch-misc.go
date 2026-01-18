package dispatcher

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/controller/storage/storagefactory"
	"meoe.io/poly-csi/pkg/transport"
)

// ==== misc ====

func (d *Dispatcher) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	dispatchParams, err := parseDispatchParams(nil, req.Parameters, nil)
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

	strInstance, err := storagefactory.StorageFromType(dispatchParams.storageType, srv.conf.Commands, srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	available, err := strInstance.GetAvailableCapacity(ctx, req.Parameters)
	if err != nil {
		return nil, fmt.Errorf("server[name=%s]: storage[%s]: GetAvailableCapacity: %w", dispatchParams.serverName, dispatchParams.storageType, err)
	}

	return &csi.GetCapacityResponse{
		AvailableCapacity: available,
	}, nil
}

// TODO
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

	ctx = transport.MakeContext(ctx, mh.srv.trInstance)

	strInstance, err := storagefactory.StorageFromType(mh.vi.StorageType, mh.srv.conf.Commands, mh.srv.conf.GlobalParams)
	if err != nil {
		return nil, err
	}

	vol, err := strInstance.ParseVolume(mh.vi.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	volumeSize, err := vol.GetSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("strInstance.GetVolumeSize: %w", err)
	}

	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      req.VolumeId,
			CapacityBytes: volumeSize,
			VolumeContext: mh.vi.VolumeContext,
		},
		Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: false,
				Message:  "ok",
			},
		},
	}, nil
}

// TODO
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

	vol, err := strInstance.ParseVolume(mh.vi.StorageData)
	if err != nil {
		return nil, fmt.Errorf("parse storage data: %w", err)
	}

	newSize := req.CapacityRange.RequiredBytes

	volumeSize, err := vol.Expand(ctx, newSize)
	if err != nil {
		return nil, fmt.Errorf("storage: expand volume: %w", err)
	}
	logger.Info("storage.Expand: success")

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         volumeSize,
		NodeExpansionRequired: true,
	}, nil
}

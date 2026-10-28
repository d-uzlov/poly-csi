package dispatcher

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"meoe.io/poly-csi/pkg/api/volumecontext"
	"meoe.io/poly-csi/pkg/controller/uuidctx"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

// type cloneSource struct {
// 	srv *parsedServer

// 	str  storage.Manager
// 	snap storage.Snapshot
// }

// func (dc *dispatchCreate) getExistingSnapshotForClone(ctx context.Context, sh *snapshotManagementHandle) (*cloneSource, error) {
// 	sourceStr, err := filevolfactory.StorageFromType(sh.srv.trInstance, sh.si.StorageType, sh.srv.Conf.GlobalParams)
// 	if err != nil {
// 		return nil, fmt.Errorf("get source storage: %w", err)
// 	}

// 	snap, err := sourceStr.ParseSnapshot(sh.si.SerializedData)
// 	if err != nil {
// 		return nil, fmt.Errorf("parse source snapshot data: %w", err)
// 	}

// 	return &cloneSource{
// 		srv:  sh.srv,
// 		str:  sourceStr,
// 		snap: snap,
// 	}, nil
// }

// func (dc *dispatchCreate) makeSnapshotForClone(ctx context.Context, sh *volumeManagementHandle) (*cloneSource, error) {
// 	sourceStr, err := filevolfactory.StorageFromType(sh.srv.trInstance, sh.vi.Storage.StorageType, sh.srv.Conf.GlobalParams)
// 	if err != nil {
// 		return nil, fmt.Errorf("get source storage: %w", err)
// 	}

// 	sourceVol, err := sourceStr.ParseVolume(sh.vi.Storage.StorageData)
// 	if err != nil {
// 		return nil, fmt.Errorf("parse source storage data: %w", err)
// 	}

// 	sourceCaps := sourceStr.GetCapabilities()
// 	if !sourceCaps.NativeSnapshot {
// 		return nil, fmt.Errorf("source volume does not support native snapshots")
// 	}

// 	tmpSnapshotId := "snap-for-" + dc.rawName
// 	sourceSnapshot, err := sourceVol.MakeSnapshot(ctx, tmpSnapshotId)
// 	if err != nil {
// 		return nil, fmt.Errorf("create source snapshot: %w", err)
// 	}
// 	// TODO cleanup should use a different context, with extended timeout
// 	defer sourceSnapshot.Delete(ctx)

// 	return &cloneSource{
// 		srv:  sh.srv,
// 		str:  sourceStr,
// 		snap: sourceSnapshot,
// 	}, nil
// }

func (d *Dispatcher) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (resp *csi.CreateVolumeResponse, finalErr error) {
	// logger := logs.FromCtx(ctx)
	conf := FromCtx(ctx)
	callUuid := uuidctx.FromCtx(ctx)

	if req.VolumeCapabilities[0].GetBlock() != nil {
		return nil, fmt.Errorf("raw block devices are not supported")
	}

	if req.VolumeCapabilities[0].GetMount() == nil {
		return nil, fmt.Errorf("fs access mode is not specified")
	}

	parser := serialmaps.NewParser(req.Parameters, serialmaps.WithRemove(true))

	dispatchMode := parser.Take("dispatch.mode")
	var helper VolumeDriver
	switch dispatchMode {
	case "block-fs":
		helper = NewDispatchBlockFs(conf, callUuid)
	case "native-fs":
		helper = NewDispatchNativeFs(conf, callUuid)
	default:
		return nil, fmt.Errorf("dispatch.mode: '%s'", dispatchMode)
	}

	err := helper.ParseRequest(req, parser)
	if err != nil {
		return nil, fmt.Errorf("helper.parseRequest: %w", err)
	}
	err = helper.InitHandles()
	if err != nil {
		return nil, fmt.Errorf("helper.initHandles: %w", err)
	}
	err = helper.Create(ctx)
	if err != nil {
		return nil, fmt.Errorf("helper.create: %w", err)
	}

	volumeContext := helper.GetVolumeContext()

	err = volumeContext.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate volume context: %w", err)
	}
	vcMap, err := serialmaps.Serialize(volumeContext, volumecontext.Prefix)
	if err != nil {
		return nil, fmt.Errorf("serialize volume context: %w", err)
	}

	volumeId := helper.MakeVolumeId()

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeId,
			CapacityBytes: helper.GetSize(),
			VolumeContext: vcMap,
			ContentSource: req.VolumeContentSource,
		},
	}, nil

	// if vi.Storage.DataSource == "" {
	// 	if req.VolumeContentSource != nil {
	// 		var clone *cloneSource
	// 		if src := req.VolumeContentSource.GetSnapshot(); src != nil {
	// 			sh, err := d.getSnapshotHandle(ctx, src.SnapshotId)
	// 			if err != nil {
	// 				return nil, fmt.Errorf("getSnapshotHandle: %w", err)
	// 			}
	// 			defer sh.release()
	// 			if sh.si == nil {
	// 				return nil, fmt.Errorf("source snapshot is not found")
	// 			}
	// 			clone, err = dc.getExistingSnapshotForClone(ctx, sh)
	// 			if err != nil {
	// 				return nil, fmt.Errorf("getExistingSnapshotForClone: %w", err)
	// 			}
	// 			vi.Storage.DataSource = src.SnapshotId
	// 		} else if src := req.VolumeContentSource.GetVolume(); src != nil {
	// 			sh, err := d.getExistingVolumeHandle(ctx, src.VolumeId)
	// 			if err != nil {
	// 				return nil, fmt.Errorf("get volume info: %w", err)
	// 			}
	// 			defer sh.release()
	// 			if sh.vi == nil {
	// 				return nil, fmt.Errorf("source volume is not found")
	// 			}

	// 			clone, err = dc.makeSnapshotForClone(ctx, sh)
	// 			if err != nil {
	// 				return nil, fmt.Errorf("makeSnapshotForClone: %w", err)
	// 			}
	// 			vi.Storage.DataSource = src.VolumeId
	// 		} else {
	// 			return nil, fmt.Errorf("unknown volume content source type: %#v", req.VolumeContentSource.Type)
	// 		}
	// 		if clone == nil {
	// 			return nil, fmt.Errorf("get source snapshot: nil result")
	// 		}
	// 		err = dc.createFromSnapshot(ctx, *clone)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("createFromSnapshot: %w", err)
	// 		}
	// 	} else {
	// 		err = dc.createFromScratch(ctx)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("createFromScratch: %w", err)
	// 		}
	// 		vi.Storage.NeedsControllerFormat = true
	// 		vi.Storage.DataSource = "scratch"
	// 	}
	// }

}

func (d *Dispatcher) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	conf := FromCtx(ctx)

	mh, err := d.getExistingVolumeHandle(ctx, req.VolumeId, conf)
	if err != nil {
		return nil, fmt.Errorf("get volume info: %w", err)
	}
	if mh == nil {
		return &csi.DeleteVolumeResponse{}, nil
	}

	err = mh.Destroy(ctx)
	if err != nil {
		return nil, err
	}

	return &csi.DeleteVolumeResponse{}, nil
}

package dispatcher

// import (
// 	"context"
// 	"fmt"

// 	"github.com/container-storage-interface/spec/lib/go/csi"
// 	"meoe.io/poly-csi/pkg/controller/filevol/filevolfactory"
// 	"meoe.io/poly-csi/pkg/controller/voldbwrapper"
// )

// func (d *Dispatcher) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
// 	mh, err := d.getExistingVolumeHandle(ctx, req.SourceVolumeId)
// 	if err != nil {
// 		return nil, fmt.Errorf("get volume info: %w", err)
// 	}
// 	defer mh.release()
// 	if mh.vi == nil {
// 		return nil, fmt.Errorf("source volume not found")
// 	}

// 	snapshotId := createSnapshotId(mh.srv.Conf.StableId, mh.volumeName, req.Name)

// 	strInstance, err := filevolfactory.StorageFromType(mh.srv.trInstance, mh.vi.Storage.StorageType, mh.srv.Conf.GlobalParams)
// 	if err != nil {
// 		return nil, err
// 	}

// 	vol, err := strInstance.ParseVolume(mh.vi.Storage.StorageData)
// 	if err != nil {
// 		return nil, fmt.Errorf("parse storage data: %w", err)
// 	}

// 	volumeState, err := vol.ReadState(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("ReadState: %w", err)
// 	}
// 	if volumeState == nil {
// 		return nil, fmt.Errorf("ReadState: nil result")
// 	}

// 	snap, err := vol.MakeSnapshot(ctx, req.Name)
// 	if err != nil {
// 		return nil, fmt.Errorf("create snapshot in storage: %w", err)
// 	}

// 	snapData, err := snap.Serialize()
// 	if err != nil {
// 		return nil, fmt.Errorf("serialize snapshot: %w", err)
// 	}

// 	err = mh.wrapper.SetSnapshotData(ctx, req.Name, &voldbwrapper.SnapshotInfo{
// 		SnapshotType:   "native",
// 		StorageType:    mh.vi.Storage.StorageType,
// 		SerializedData: snapData,
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("save snapshot data: %w", err)
// 	}

// 	return &csi.CreateSnapshotResponse{
// 		Snapshot: &csi.Snapshot{
// 			SnapshotId:     snapshotId,
// 			SourceVolumeId: req.SourceVolumeId,
// 			CreationTime:   nil, // TODO
// 			SizeBytes:      volumeState.Size,
// 			ReadyToUse:     true,
// 		},
// 	}, nil
// }

// func (d *Dispatcher) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
// 	mh, err := d.getSnapshotHandle(ctx, req.SnapshotId)
// 	if err != nil {
// 		return nil, fmt.Errorf("get snapshot handle: %w", err)
// 	}
// 	defer mh.release()
// 	if mh.si == nil {
// 		return nil, fmt.Errorf("snapshot not found")
// 	}

// 	strInstance, err := filevolfactory.StorageFromType(mh.srv.trInstance, mh.si.StorageType, mh.srv.Conf.GlobalParams)
// 	if err != nil {
// 		return nil, err
// 	}

// 	snap, err := strInstance.ParseSnapshot(mh.si.SerializedData)
// 	if err != nil {
// 		return nil, fmt.Errorf("parse snapshot data: %w", err)
// 	}

// 	err = snap.Delete(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("delete underlying snapshot: %w", err)
// 	}

// 	err = mh.wrapper.DeleteSnapshotData(ctx, mh.snapshotName)
// 	if err != nil {
// 		return nil, fmt.Errorf("delete snapshot data from database: %w", err)
// 	}

// 	return &csi.DeleteSnapshotResponse{}, nil
// }

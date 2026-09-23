package storage

import (
	"context"
	"time"
)

type StorageManager interface {
	GetCapabilities() Capabilities
	GetAvailableCapacity(ctx context.Context, staticParams map[string]string) (int64, error)
	// CreateVolume creates a new volume handle.
	// It does not check or change underlying storage.
	CreateVolume(req CreateRequest) (Volume, error)
	// ParseVolume recreates volume handle from result of Volume.Serialize
	ParseVolume(serialized string) (Volume, error)
	// ParseSnapshot recreates snapshot handle from result of Snapshot.Serialize
	ParseSnapshot(serialized string) (Snapshot, error)
}

type Volume interface {
	// Serialize returns a string that can be used in StorageManager.ParseVolume to recreate this volume handle
	Serialize() (string, error)

	// CreateState configures underlying storage
	// It may return an error if matching volume already exists,
	// so you need to check that ReadState() is (nil, nil)
	//
	// If an error happens, storage could be partially configured.
	// Cleanup is expected to be managed by caller.
	CreateState(ctx context.Context, size int64) error
	// ReadState returns volume state, or (nil, nil) if volume does not exist in underlying storage
	ReadState(ctx context.Context) (*VolumeState, error)
	DeleteState(ctx context.Context) error

	// Modify(ctx context.Context, req *csi.ControllerModifyVolumeRequest) error

	SetSize(ctx context.Context, newSize int64) error

	// MakeSnapshot must create an instantaneous consistent snapshot or return an error
	MakeSnapshot(ctx context.Context, name string) (Snapshot, error)
	// ReceiveSnapshot initializes a new volume from snapshot content.
	// sourceFile must be a pipe that is fed using Snapshot.Send from StorageManager with matching StorageManager.GetCapabilities.LocalSendType
	//
	// If an error happens, storage could be partially configured.
	// Cleanup is expected to be managed by caller.
	ReceiveSnapshot(ctx context.Context, sourceFile string, timeout time.Duration) error
}

type Snapshot interface {
	Serialize() (string, error)

	// GetLocalPath returns path in local filesystem containing snapshot content
	// The path should be read-only if possible.
	// Some implementations may need to take additional actions to make snapshot accessible in the filesystem,
	// which requires caller to invoke "release" function when they are done.
	// Release function must be safe to run several times.
	// If path is not empty, release must not be nil.
	GetLocalPath(ctx context.Context) (path string, release func(), err error)
	Send(ctx context.Context, targetFile string, timeout time.Duration) error
	Delete(ctx context.Context) error
}

type CreateRequest struct {
	Id            string
	StaticParams  map[string]string
	MutableParams map[string]string
}

type Capabilities struct {
	// LocalPathType holds type of volume in filesystem.
	// It must be one of well-known types.
	// If volume is not available in filesystem, LocalPathType must be empty.
	// Dispatcher can use this info for filesystem checks.
	// Filesystem-based share methods can use this to expose volume to network.
	LocalPathType string

	// OpaqueVolumeType holds storage-specific volume type.
	// Unlike LocalPathType, OpaqueVolumeType does not use well-known values.
	// Storage-specific share methods can use this to expose volume to network.
	OpaqueVolumeType string

	// Indicates whether volumes in this storage support instantaneous online snapshots.
	NativeSnapshot bool

	// LocalSendType holds type of protocol used to send (and receive) volume in pipe.
	// Since only snapshots can be sent, LocalSendType is meaningless if native snapshots are disabled on storage.
	// When 2 storage managers report the same LocalSendType method,
	// dispatcher can use it for optimized storage-specific volume cloning.
	LocalSendType string

	// BlockInitialEmpty indicates that right after initial volume allocation
	// block device can be safely used without discard and/or zeroing.
	// Ignored for non-block devices.
	BlockInitialEmpty bool
}

// well-known local path types
const (
	VolFolder = "vol_type_folder"
	VolDevice = "vol_type_device"
)

type VolumeState struct {
	// FsPath is the path in the Server filesystem where volume is available
	// Can be empty in case volume can not be found in filesystem.
	FsPath string
	// Size is the size limit of the volume.
	// MUST be populated correctly for block volumes.
	// For filesystem volumes that cannot enforce a size limit on a volume value must be zero.
	Size int64

	// values below are intended for optimization of block device formatting
	// they are all optional, and should be ignored by filesystem-based volumes

	// minimum write size without read-modify-write
	RMWSize int64
	// minimum size for performance
	OptimalWriteSize int64
	// maximum size that is guaranteed to atomically succeed of fail
	AtomicSize int64
}

const (
	HealthStatusDegraded     = "degraded"
	HealthStatusDataLoss     = "data_loss"
	HealthStatusInaccessible = "inaccessible"
)

// HealthMessage is a mirror of csi.VolumeHealth_VolumeHealthEntry, created to remove unnecessary dependency
type HealthMessage struct {
	Status  string
	Reason  string
	Message string
}

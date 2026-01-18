package storage

import (
	"context"
	"time"
)

type StorageManager interface {
	GetCapabilities() Capabilities
	GetAvailableCapacity(ctx context.Context, staticParams map[string]string) (int64, error)
	// GetExistingVolume checks if volume with specified parameters already exists in the underlying storage
	// returns (nil, nil) if volume does not exist
	GetExistingVolume(ctx context.Context, req CreateRequest) (Volume, error)
	// CreateVolume configures underlying storage and returns a volume handle
	// It may return an error if matching volume already exists, so you should call GetExistingVolume first
	CreateVolume(ctx context.Context, req CreateRequest) (Volume, error)
	// Receive creates a new volume based on contents of a pipe specified via sourceFile
	// The stream must be produced by a volume snapshot with the same value of StorageManager.GetCapabilities.LocalSend
	Receive(ctx context.Context, sourceFile string, req CreateRequest, timeout time.Duration) (Volume, error)
	// ParseVolume recreates volume handle from result of Volume.Serialize
	ParseVolume(serialized string) (Volume, error)
	// ParseSnapshot recreates snapshot handle from result of Snapshot.Serialize
	ParseSnapshot(serialized string) (Snapshot, error)
}

type Volume interface {
	// Serialize returns a string that can be used in StorageManager.ParseVolume to recreate this volume handle
	Serialize() (string, error)
	GetLocalPath(ctx context.Context) (string, error)
	Delete(ctx context.Context) error
	GetSize(ctx context.Context) (int64, error)
	// Modify(ctx context.Context, req *csi.ControllerModifyVolumeRequest) error
	Expand(ctx context.Context, newSize int64) (int64, error)
	// MakeSnapshot must create an instantaneous consistent snapshot or return an error
	MakeSnapshot(ctx context.Context, name string) (Snapshot, error)
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
	Size          int64
	StaticParams  map[string]string
	MutableParams map[string]string
}

type Capabilities struct {
	// LocalVolume holds type of volume in filesystem.
	// It must be one of well-known types.
	// If volume is not available in filesystem, LocalVolume must be empty.
	// Dispatcher can use this info for filesystem checks.
	// Filesystem-based share methods can use this to expose volume to network.
	LocalVolume string

	// OpaqueVolume holds storage-specific volume type.
	// Unlike LocalVolume, OpaqueVolume does not use well-known values.
	// Storage-specific share methods can use this to expose volume to network.
	OpaqueVolume string

	// Indicates whether volumes in this storage support instantaneous online snapshots.
	NativeSnapshot bool

	// LocalSend holds type of protocol used to send (and receive) volume in pipe.
	// Since only snapshots can be sent, LocalSend is meaningless if native snapshots are disabled on storage.
	// When 2 storage managers report the same LocalSend method,
	// dispatcher can use it for optimized storage-specific volume cloning.
	LocalSend string
}

// well-known local volume types
const (
	VolFolder = "vol_type_folder"
	VolDevice = "vol_type_device"
)

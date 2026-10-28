package filevol

import (
	"context"
	"encoding/json"
	"time"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type Manager interface {
	GetCapabilities() Capabilities
	GetAvailableCapacity(ctx context.Context, parser *serialmaps.Parser) (int64, error)
	// CreateVolume creates a new volume handle.
	// It does not check or change underlying storage.
	CreateVolume(req CreateRequest) (Volume, error)
	// ParseVolume recreates volume handle from result of Volume.Serialize
	ParseVolume(serialized json.RawMessage) (Volume, error)
	// ParseSnapshot recreates snapshot handle from result of Snapshot.Serialize
	ParseSnapshot(serialized json.RawMessage) (Snapshot, error)
}

type Volume interface {
	// Serialize returns a string that can be used in StorageManager.ParseVolume to recreate this volume handle
	Serialize() (json.RawMessage, error)

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
	Serialize() (json.RawMessage, error)

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
	Id     string
	Parser *serialmaps.Parser
}

type Capabilities struct {
	// OpaqueType holds storage-specific volume type.
	// Unlike LocalPathType, OpaqueType does not use well-known values.
	// Storage-specific share methods can use this to expose volume to network.
	OpaqueType OpaqueVolumeType

	// Indicates whether volumes in this storage support instantaneous online snapshots.
	NativeSnapshot bool

	// LocalSendType holds type of protocol used to send (and receive) volume in pipe.
	// Since only snapshots can be sent, LocalSendType is meaningless if native snapshots are disabled on storage.
	// When 2 storage managers report the same LocalSendType method,
	// dispatcher can use it for optimized storage-specific volume cloning.
	LocalSendType string
}

type OpaqueVolumeType string

type VolumeState struct {
	// FsPath is the path in the Server filesystem where volume is available
	// Can be empty in case volume can not be found in filesystem.
	FsPath string
	// Size is the size limit of the volume.
	// For filesystem volumes that cannot enforce a size limit on a volume value must be zero.
	Size int64
}

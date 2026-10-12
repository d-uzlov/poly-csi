package filecon

import (
	"context"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/api/fsapi"
	"meoe.io/poly-csi/pkg/transport"
)

type InflateFunc = func(conf CommonConfig) (Manager, error)
type MetadataInflateFunc = func() Metadata

type Metadata interface {
	Configure(requestedSize int64, params map[string]string) error
	RequestBlockDevices() map[string]BlockDeviceRequirement
	SetVolumeProps(props map[string]StorageProperties) error
	GetData() fsapi.Data
}

type StorageProperties struct {
	Size             int64 `json:"size"`
	InitialEmpty     bool  `json:"initial_empty"`
	RMWSize          int64 `json:"rmw_size"`
	OptimalWriteSize int64 `json:"optimal_size"`
	AtomicSize       int64 `json:"atomic_size"`
}

type BlockDeviceRequirement struct {
	MinSize int64
}

type Manager interface {
	// Capabilities returns an object describing capabilities of this type of Manager
	Capabilities(ctx context.Context) (Capabilities, error)
	// GetHandle creates a handle to manage a connection.
	// Manager must parse params and create a handle, without making any changes in OS.
	GetHandle(ctx context.Context, req ConnectRequest) (Handle, error)
	// ParseHandle recreates handle object from [Handle.Serialize] result
	ParseHandle(serializedData string) (Handle, error)
}

type ConnectRequest struct {
	Uuid                    string            `json:"uuid"`
	Proto                   fsapi.Data        `json:"proto"`
	DisableCleanupOnFailure bool              `json:"disableCleanupOnFailure"`
	Preset                  map[string]string `json:"preset"`
}

type CommonConfig struct {
	Tr  transport.Transport
	Cfg yaml.Node
}

type Capabilities struct {
	FsType fsapi.Name

	// HasNodeInfo indicates if GetInfo of this connector provides essential info needed to create shares
	HasNodeInfo bool `json:"has_node_info"`

	// ConnectType specifies the how connection should be created.
	// See constants below for more info
	ConnectType string `json:"connect_type"`

	// ManagedUnmount requests the caller to call unmount on connected folder before Disconnect.
	// This is ignored when ConnectType is ConnectTypeBlock.
	SpecialMount bool `json:"special_mount"`
}

// constants for Capabilities.ConnectType
const (
	// connect mounts a folder during staging
	ConnectTypeFolder = "__connect_type_folder"
	// connect mounts a folder during publishing
	// this also implies that Disconnect is never called, folder will be automatically unmounted.
	ConnectTypePublishFolder = "__connect_type_publish_folder"
)

type Handle interface {
	// Prepare runs global volume setup.
	// This can include block device format, fetching configs, etc.
	// If ([Capabilities.SpecialMount] == false) Prepare MUST ensure
	// that filesystem is mounted at [PrepareRequest.TargetPath].
	Prepare(ctx context.Context, req PrepareRequest) error

	// Mount must ensure that filesystem is mounted at [MountRequest.TargetPath].
	// If already mounted, this is a no-op.
	Mount(ctx context.Context, req MountRequest) error
	Unmount(ctx context.Context, path string) error

	// Disconnect must ensure that volume is disconnected.
	// If system does not have volume info, disconnect is a no-op without errors.
	Disconnect(ctx context.Context) error

	// Expand ensures that filesystem size grows to maximum size available to the volume.
	Expand(ctx context.Context) error

	CheckHealth(ctx context.Context) ([]HealthMessage, error)

	// Serialize creates an opaque string that can be used in [Manager.ParseHandle]
	Serialize() (string, error)
	Validate() error
}

type PrepareRequest struct {
	Devices     map[string]string `json:"devices"`
	StagingPath string            `json:"stagingPath"`
	Flags       []string          `json:"mountFlags"`
}

type NoPrepare struct{}

func (i *NoPrepare) Prepare(ctx context.Context, req PrepareRequest) error {
	return nil
}

type MountRequest struct {
	StagingPath string   `json:"stagingPath"`
	TargetPath  string   `json:"targetPath"`
	Flags       []string `json:"mountFlags"`
	FsGroup     string   `json:"mountGroup"`
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

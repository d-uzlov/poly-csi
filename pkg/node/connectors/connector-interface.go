package connectors

import (
	"context"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/addropt"
	"meoe.io/poly-csi/pkg/transport"
)

type InflateFunc = func(conf CommonConfig) (Manager, error)

type Manager interface {
	// Capabilities returns an object describing capabilities of this type of connector
	Capabilities(ctx context.Context) (Capabilities, error)
	// GetInfo returns opaque string containing connector-specific info about the node.
	// Controller can parse this info and use it to publish the volume.
	GetInfo(ctx context.Context) (*ConnectorNodeInfo, error)
	// GetHandle creates a handle to manage a connection.
	// Connector must parse params and create a handle, without making any changes in OS.
	GetHandle(ctx context.Context, req ConnectRequest) (Handle, error)
	// ParseHandle recreates state object from State.Serialize result
	ParseHandle(serializedData string) (Handle, error)
}

type Handle interface {
	ConnectBlock(ctx context.Context) error
	// GetDevices returns list of block devices associated connection.
	// It's irrelevant for connectors that don't create block devices.
	// The returned list contains devices with the same storage backend (for external multipath setup).
	// Index can enumerate independent devices, starting with 0.
	GetDevices(ctx context.Context, index int) ([]string, error)

	ConnectFolder(ctx context.Context, req ConnectFolderRequest) error

	// Disconnect must ensure that volume is disconnected.
	// If system does not have volume info, disconnect is a no-op without errors.
	Disconnect(ctx context.Context) error
	// Expand ensures that local state synchronizes with remote state.
	Expand(ctx context.Context) error

	CheckHealth(ctx context.Context) ([]HealthMessage, error)

	// Serialize creates an opaque string that can be used in [Manager].ParseState
	Serialize() (string, error)
	Validate() error
}

type CommonConfig struct {
	Tr        transport.Transport
	DevFolder string
	Cfg       yaml.Node
}

type Capabilities struct {
	// HasNodeInfo indicates if GetInfo of this connector provides essential info needed to create shares
	HasNodeInfo bool `json:"has_node_info"`

	// ConnectType specifies the how connection should be created.
	// See constants below for more info
	ConnectType string `json:"connect_type"`

	// ManagedUnmount requests the caller to call unmount on connected folder before Disconnect.
	// This is ignored when ConnectType is ConnectTypeBlock.
	ManagedUnmount bool `json:"managed_unmount"`

	// ContextPrefix indicated which keys in CSI publish context are part of the connector config.
	ContextPrefix string `json:"context_prefix"`
}

// constants for Capabilities.ConnectType
const (
	// connect provides block device(s)
	ConnectTypeBlock = "__connect_type_block"
	// connect mounts a folder during staging
	ConnectTypeFolder = "__connect_type_folder"
	// connect mounts a folder during publishing
	// this also implies that Disconnect is never called, folder will be automatically unmounted.
	ConnectTypePublishFolder = "__connect_type_publish_folder"
)

type ConnectRequest struct {
	AddressOptions []addropt.AddressOption `json:"address_options"`
	VolumeContext  map[string]string       `json:"volume_context"`
	PublishContext map[string]string       `json:"publish_context"`
	// PublishSecret is a decrypted secret from publish context
	PublishSecret           []byte            `json:"publish_context_secret"`
	DisableCleanupOnFailure bool              `json:"disableCleanupOnFailure"`
	Preset                  map[string]string `json:"preset"`
}

type ConnectFolderRequest struct {
	FsMountPath string   `json:"fs_mount_path"`
	FsType      string   `json:"fs_type"`
	MountFlags  []string `json:"mount_flags"`
	MountGroup  string   `json:"mount_group"`
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

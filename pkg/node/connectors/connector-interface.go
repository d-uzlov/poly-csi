package connectors

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/node/addropt"
	"meoe.io/poly-csi/pkg/transport"
)

type Connector interface {
	Capabilities(ctx context.Context) (Capabilities, error)
	GetInfo(ctx context.Context) (*ConnectorNodeInfo, error)
	Configure(ctx context.Context, tr transport.Transport, preset map[string]string) error
	Connect(ctx context.Context, req ConnectRequest) (*ConnectResult, error)
	Disconnect(ctx context.Context, serializedData string) error
	Expand(ctx context.Context, serializedData string) error
}

type Capabilities struct {
	// ConnectType specifies the how Connector.Connect should be used.
	// See constants below for more info
	ConnectType string `json:"connect_type"`

	// ManagedUnmount requests the caller to call unmount on connected folder before Disconnect.
	// This is ignored when ConnectType is ConnectTypeBlock.
	ManagedUnmount bool `json:"managed_unmount"`
}

// constants for Capabilities.ConnectType
const (
	// connect provides block device
	ConnectTypeBlock = "__connect_type_block"
	// connect mounts a folder during staging
	ConnectTypeFolder = "__connect_type_folder"
	// connect mounts a folder during publishing
	// this also implies that Disconnect is never called, folder will be automatically unmounted.
	ConnectTypePublishFolder = "__connect_type_publish_folder"
)

type ConnectRequest struct {
	// FsMountPath is the suggested path for filesystem mount.
	// When Capabilities.MountType is MountTypeFolder, node dispatcher expects a mountpoint at this path after successful Connector.Connect
	FsMountPath    string                            `json:"fs_mount_path"`
	AddressOptions []addropt.AddressOption           `json:"address_options"`
	VolumeContext  map[string]string                 `json:"volume_context"`
	PublishContext map[string]string                 `json:"publish_context"`
	AccessMount    *csi.VolumeCapability_MountVolume `json:"access_mount"`
	// PublishSecret is a decrypted secret from publish context
	PublishSecret           []byte `json:"publish_context_secret"`
	DisableCleanupOnFailure bool   `json:"disableCleanupOnFailure"`
}

type ConnectResult struct {
	// Devices is the list of block devices representing the volume.
	// It is only used when Capabilities.ConnectType is ConnectTypeBlock.
	// When the list contains more than one device, node dispatcher will use multipathd to set up united device.
	Devices []string `json:"device_path"`

	// ConnectorData and SerializedConnectorData are mutually exclusive options for connector data storage.
	// When ConnectorData is set, dispatcher will try to use JSON serialization to create SerializedConnectorData.
	ConnectorData any `json:"-"`
	// SerializedConnectorData is data passed to later requests to connector.
	// It can be automatically populated from ConnectorData.
	SerializedConnectorData string `json:"connector_data"`
}

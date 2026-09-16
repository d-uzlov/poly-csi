package connectors

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/node/addropt"
	"meoe.io/poly-csi/pkg/transport"
)

type InflateFunc = func() Connector

type Connector interface {
	// Configure sets universal parameters. It MUST be the first method called on the connector.
	// It can be considered a part of the constructor, which is moved to a method for unified initialization.
	// opaqueConfig is a YAML-encoded config with unspecified structure
	Configure(ctx context.Context, conf CommonConfig, opaqueConfig []byte) error
	// Capabilities returns an object describing capabilities of this type of connector
	Capabilities(ctx context.Context) (Capabilities, error)
	// GetInfo returns opaque string containing connector-specific info about the node.
	// Controller can parse this info and use it to publish the volume.
	GetInfo(ctx context.Context) (*ConnectorNodeInfo, error)
	// Connect creates node state representing the volume.
	// It returns a State object which can manage this state.
	Connect(ctx context.Context, req ConnectRequest) (State, error)
	// ParseState recreates state object from State.Serialize result
	ParseState(serializedData string) (State, error)
}

type State interface {
	// GetDevices returns list of block devices associated with serializedData.
	// It's irrelevant for connectors that don't create block devices.
	GetDevices(ctx context.Context) ([]string, error)
	// Disconnect destroys state associated with serializedData.
	Disconnect(ctx context.Context) error
	// Expand ensures that local state synchronizes with remote state.
	Expand(ctx context.Context) error
	// Serialize creates an opaque string that can be used in Connector.ParseState
	Serialize() (string, error)
	Validate() error
}

type CommonConfig struct {
	Tr        transport.Transport
	DevFolder string
	Preset    map[string]string
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
	// connect provides block device(s)
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
	// ConnectorData and SerializedConnectorData are mutually exclusive options for connector data storage.
	// When ConnectorData is set, dispatcher will try to use JSON serialization to create SerializedConnectorData.
	ConnectorData any `json:"-"`
	// SerializedConnectorData is data passed to later requests to connector.
	// It can be automatically populated from ConnectorData.
	SerializedConnectorData string `json:"connector_data"`
}

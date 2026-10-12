package blockcon

import (
	"context"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/transport"
)

type InflateFunc = func(conf CommonConfig) (Manager, error)

type Manager interface {
	// Capabilities returns an object describing capabilities of this type of Manager
	Capabilities(ctx context.Context) (Capabilities, error)
	// GetInfo returns opaque string containing connector-specific info about the node.
	// Controller can parse this info and use it to publish the volume.
	GetInfo(ctx context.Context) (blockapi.NodeInfo, error)
	// GetHandle creates a handle to manage a connection.
	// Manager must parse params and create a handle, without making any changes in OS.
	GetHandle(ctx context.Context, req ConnectRequest) (Handle, error)
	// ParseHandle recreates handle object from [Handle.Serialize] result
	ParseHandle(serializedData string) (Handle, error)
}

type NoInfo struct{}

func (i *NoInfo) GetInfo(ctx context.Context) (blockapi.NodeInfo, error) {
	return nil, nil
}

type Handle interface {
	Connect(ctx context.Context) error
	// GetDevices returns list of block devices associated with connection.
	// The returned list contains devices with the same backend volume (for external multipath setup).
	GetDevices(ctx context.Context) ([]string, error)

	// Disconnect must ensure that volume is disconnected.
	// If volume is not connected, disconnect is a no-op without errors.
	Disconnect(ctx context.Context) error
	// Expand ensures that local state synchronizes with remote state.
	Expand(ctx context.Context) error

	CheckHealth(ctx context.Context) ([]HealthMessage, error)

	// Serialize creates an opaque string that can be used in [Manager.ParseHandle]
	Serialize() (string, error)
	Validate() error
}

type CommonConfig struct {
	Tr        transport.Transport
	DevFolder string
	Cfg       yaml.Node
}

type Capabilities struct {
	Protocol blockapi.Name

	// HasNodeInfo indicates if GetInfo of this connector provides essential info needed to create shares
	HasNodeInfo bool `json:"has_node_info"`
}

type ConnectRequest struct {
	Uuid                    string                         `json:"uuid"`
	Proto                   blockapi.Data                  `json:"proto"`
	DisableCleanupOnFailure bool                           `json:"disableCleanupOnFailure"`
	Preset                  map[string]string              `json:"preset"`
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

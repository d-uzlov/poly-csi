package mounts

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/node/addropt"
)

const (
	MountIdKey = "mounter_id"
)

type NodeMounter interface {
	GetInfo(ctx context.Context) (*MounterNodeInfo, error)
	Connect(ctx context.Context, req ConnectRequest) (*ConnectResult, error)
	Disconnect(ctx context.Context, serializedMounterData string) error
	Expand(ctx context.Context, serializedMounterData string) error
}

// ConnectRequest is mostly equivalent to csi.NodeStageVolumeRequest
// except this struct can be (de)serialized without errors
type ConnectRequest struct {
	AddressOptions       []addropt.AddressOption           `json:"address_options"`
	VolumeContext        map[string]string                 `json:"volume_context"`
	PublishContext       map[string]string                 `json:"publish_context"`
	StagingTargetPath    string                            `json:"staging_target_path"`
	AccessBlock          *csi.VolumeCapability_BlockVolume `json:"access_block"`
	AccessMount          *csi.VolumeCapability_MountVolume `json:"access_mount"`
	Secrets              map[string]string                 `json:"secrets"`
	PublishContextSecret map[string]string                 `json:"publish_context_secret"`
}

type ConnectResult struct {
	// FolderPath is path to folder with mounted filesystem.
	// If ManagedOps.MountFs is set, the value is optional, node dispatcher will try to mount filesystem from device(s).
	FolderPath string `json:"folder_path"`
	// Devices is zero, one or several paths in filesystem pointing to devices with volume data.
	// If FolderPath is not empty, the Devices list should be empty.
	// If FolderPath is empty, Devices must contain at least one value AND ManagedOps.MountFs must be set.
	// When the list contains more than one device, node dispatcher will use multipathd to set up united device.
	Devices []string `json:"device_path"`
	// ManagedOps is a set of requests from mounter to node dispatcher
	// which allows mounter to skip various generic operations.
	ManagedOps ManagedOperations `json:"managed_operations"`

	// MounterData and SerializedMounterData are mutually exclusive options for mounter data storage.
	// When MounterData is set, dispatcher will try to use JSON serialization to create SerializedMounterData.
	MounterData any `json:"-"`
	// SerializedMounterData is data passed to later requests to mounter.
	// It can be automatically populated from MounterData.
	SerializedMounterData string `json:"mounter_data"`
}

type ManagedOperations struct {
	// request dispatcher to mount DevicePath as folder
	// implies formatting if filesystem does not exist yet
	MountFs bool `json:"mount"`
	// request dispatcher to unmount FolderPath
	// 	can be used even when MountFs is false
	UnmountFs bool `json:"unmount"`
	// request dispatcher to resize filesystem to fit new device size
	ExpandFs bool `json:"expand"`
}

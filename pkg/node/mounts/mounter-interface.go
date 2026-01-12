package mounts

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/node/addropt"
)

type NodeMounter interface {
	GetInfo(ctx context.Context) (*MounterNodeInfo, error)
	Connect(ctx context.Context, req ConnectRequest) (*MounterContext, error)
	Disconnect(ctx context.Context, mc MounterContext) error
	Expand(ctx context.Context, mc MounterContext) error
}

// mostly equivalent to csi.NodeStageVolumeRequest
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

const (
	MountIdKey = "mounter_id"
)

type MounterContext struct {
	// optional path to folder with mounted filesystem
	FolderPath string `json:"folder_path"`
	// if FolderPath is not set, mounter can set DevicePath and set ManagedOps
	// to request node dispatcher to mount filesystem
	Devices    []string          `json:"device_path"`
	ManagedOps ManagedOperations `json:"managed_operations"`
	// extra is any info that mounter wants to get in later operations
	Extra map[string]string `json:"extra"`
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

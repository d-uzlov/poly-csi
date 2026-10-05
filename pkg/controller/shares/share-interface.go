package shares

import (
	"context"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/protocols"
)

type ShareManager interface {
	GetCapabilities() Capabilities
	// CreateShare parses request parameters and creates a handle that can be used to actually manage the share.
	CreateShare(req CreateShareRequest) (Share, error)
	// ParseShare recreates share handle from result of Share.Serialize
	ParseShare(shareData string) (Share, error)
}

type Share interface {
	Validate() error
	// PrepareShare configures a skeleton for a share.
	PrepareShare(ctx context.Context) error
	// AttachPath configures sharing of an object in the filesystem.
	// If Capabilities.MultiShare is enabled, can be called multiple times, but the first call MUST be with index=0
	AttachPath(ctx context.Context, fsPath string, index int) error
	// AttachVolume configures sharing of a volume, when some storage-specific info is required.
	// Implementation can cast the volume object to expected type to extract info from it.
	AttachVolume(ctx context.Context, volume any, index int) error
	Publish(ctx context.Context, req PublishRequest) (*PublishContext, error)
	Unpublish(ctx context.Context, serializedLocalData string) error
	UnpublishAll(ctx context.Context) error
	DeleteShare(ctx context.Context) error
	Serialize() (string, error)
}

type Capabilities struct {
	ProtocolName protocols.Name
	// UnpublishAll indicates whether this ShareManager implements an optimized UnpublishAll method.
	// When false, dispatcher will call Unpublish for each published node instead of attempting to use UnpublishAll.
	UnpublishAll bool
	// LocalVolume indicates which types of volumes are accepted for this share, matching storage.StorageManager.GetCapabilities.LocalVolume.
	// When empty, this check is skipped.
	LocalVolume storage.LocalVolumeType
	// OpaqueVolume indicates which types of volumes are accepted for this share, matching storage.StorageManager.GetCapabilities.OpaqueVolume.
	// When empty, this check is skipped.
	OpaqueVolume storage.OpaqueVolumeType
	// MultiShare indicates that Share.SharePath accept the index parameter
	// and can be called multiple times. All shared paths/volumes will be connected via a single connection.
	MultiShare bool
}

type CreateShareRequest struct {
	// ShareId should be used as the main id for this share, if possible.
	// It is user-supplied, so it helps the user identify share-related resources
	// when accessing them for any kind of maintenance or debugging.
	ShareId string
	// StaticParams is an opaque set of parameters specifying the share properties
	StaticParams map[string]string
	// MutableParams override StaticParams.
	// Implementation must fail if MutableParams contains something that can not be dynamically changed.
	MutableParams map[string]string
}

type PublishRequest struct {
	NodeName string
	ConInfo  protocols.NodeInfo
}

type PublishContext struct {
	// Protocol data is passed to node connector
	Protocol protocols.Config `json:"public"`
	// LocalData is an arbitrary storage for internal information for ShareManager.Publish and ShareManager.Unpublish.
	// Can be empty/nil if ShareManager.Unpublish is a no-op.
	// It is serialized into JSON, annotate it accordingly.
	LocalData any `json:"localData"`
}

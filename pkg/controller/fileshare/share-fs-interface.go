package fileshare

import (
	"context"
	"encoding/json"

	"meoe.io/poly-csi/pkg/api/fsapi"
	"meoe.io/poly-csi/pkg/controller/filevol"
)

type Manager interface {
	GetCapabilities() Capabilities
	// CreateShare parses request parameters and creates a handle that can be used to actually manage the share.
	CreateShare(req CreateShareRequest) (Share, error)
	// ParseShare recreates share handle from result of Share.Serialize
	ParseShare(shareData json.RawMessage) (Share, error)
}

type Share interface {
	Validate() error
	// PrepareShare configures a skeleton for a share.
	PrepareShare(ctx context.Context) error
	// AttachPath configures sharing of an object in the filesystem.
	AttachPath(ctx context.Context, fsPath string) error
	// AttachVolume configures sharing of a volume, when some storage-specific info is required.
	// Implementation can cast the volume object to expected type to extract info from it.
	AttachVolume(ctx context.Context, volume any) error
	Publish(ctx context.Context, req PublishRequest) (*PublishContext, error)
	Unpublish(ctx context.Context, serializedLocalData string) error
	UnpublishAll(ctx context.Context) error
	DeleteShare(ctx context.Context) error
	Serialize() (json.RawMessage, error)
}

type Capabilities struct {
	ProtocolName fsapi.Name
	// UnpublishAll indicates whether this ShareManager implements an optimized UnpublishAll method.
	// When false, dispatcher will call Unpublish for each published node instead of attempting to use UnpublishAll.
	UnpublishAll bool
	// OpaqueVolume indicates which types of volumes are accepted for this share, matching storage.StorageManager.GetCapabilities.OpaqueVolume.
	// When empty, this check is skipped.
	OpaqueVolume filevol.OpaqueVolumeType
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
	NodeName    string
	AddressList []string
}

type PublishContext struct {
	// Protocol data is passed to node connector
	Protocol fsapi.Data
	// LocalData is serialized into JSON and then used on [Share.Unpublish]
	LocalData any
}

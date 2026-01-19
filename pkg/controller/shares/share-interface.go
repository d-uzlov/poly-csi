package shares

import (
	"context"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/node/mounts"
)

type ShareManager interface {
	GetCapabilities() Capabilities
	CreateShare(ctx context.Context, req CreateShareRequest) (shareData string, err error)
	Publish(ctx context.Context, shareData string, mi mounts.MounterNodeInfo) (*PublishContext, error)
	Unpublish(ctx context.Context, shareData string, serialized string) error
	UnpublishAll(ctx context.Context, shareData string) error
	DeleteShare(ctx context.Context, shareData string) error
}

type CreateShareRequest struct {
	ShareId       string
	StaticParams  map[string]string
	MutableParams map[string]string
	LocalVolume   string
	OpaqueVolume  string
	Volume        storage.Volume
}

type PublishContext struct {
	// Public map is passed to node deployment as cleartext
	// Users (both human and automated controllers) can inspect it.
	Public map[string]string
	// Secret map is similar to Public, but it's encrypted
	Secret map[string]string
	// Serialized is an arbitrary string created in ShareManager.Publish.
	// It's the main source of info for ShareManager.Unpublish
	Data PublishData
}

type PublishData interface {
	Serialize() (string, error)
}

type Capabilities struct {
	// UnpublishAll indicates whether this ShareManager implements an optimized UnpublishAll method.
	// When false, dispatcher will call Unpublish for each published node instead of attempting to use UnpublishAll.
	UnpublishAll bool
	// LocalVolume indicates which types of volumes are accepted for this share, matching storage.StorageManager.GetCapabilities.LocalVolume.
	// When empty, this check is skipped.
	LocalVolume []string
	// OpaqueVolume indicates which types of volumes are accepted for this share, matching storage.StorageManager.GetCapabilities.OpaqueVolume.
	// When empty, this check is skipped.
	OpaqueVolume []string
}

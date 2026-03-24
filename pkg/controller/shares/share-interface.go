package shares

import (
	"context"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/node/connectors"
)

type ShareManager interface {
	GetCapabilities() Capabilities
	CreateShare(ctx context.Context, req CreateShareRequest) (shareData string, err error)
	Publish(ctx context.Context, shareData string, mi connectors.ConnectorNodeInfo) (*PublishContext, error)
	Unpublish(ctx context.Context, shareData string, serialized string) error
	UnpublishAll(ctx context.Context, shareData string) error
	DeleteShare(ctx context.Context, shareData string) error
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

type CreateShareRequest struct {
	ShareId       string
	StaticParams  map[string]string
	MutableParams map[string]string
	LocalVolume   string
	OpaqueVolume  string
	Volume        storage.Volume
}

type PublishContext struct {
	// Public data is passed to node deployment as cleartext.
	// Users (both human and automated controllers) can inspect it.
	//
	// It is serialized using serialmaps package.
	// It must be a struct with fields annotated with `map:"map_key"` tags
	Public any
	// Secret data is passed to node deployment as serialized and encrypted string.
	// It is serialized into JSON, annotate it accordingly.
	Secret any
	// LocalData is an arbitrary storage for internal information for ShareManager.Publish and ShareManager.Unpublish.
	// Can be empty/nil if ShareManager.Unpublish is a no-op.
	// It is serialized into JSON, annotate it accordingly.
	LocalData any
}

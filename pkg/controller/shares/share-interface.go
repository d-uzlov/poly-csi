package shares

import (
	"context"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/node/mounts"
)

type ShareHelper interface {
	GetCapabilities() Capabilities
	CreateShare(ctx context.Context, req CreateShareRequest) (shareData string, err error)
	Publish(ctx context.Context, shareData string, mi mounts.MounterNodeInfo) (*PublishContext, error)
	Unpublish(ctx context.Context, shareData string, localCtx map[string]string) error
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
	Public map[string]string
	Secret map[string]string
	Local  map[string]string
}

type Capabilities struct {
	LocalVolume  []string
	OpaqueVolume []string
}

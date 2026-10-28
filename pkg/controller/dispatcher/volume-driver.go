package dispatcher

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"meoe.io/poly-csi/pkg/api/publishcontext"
	"meoe.io/poly-csi/pkg/api/volumecontext"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type VolumeDriver interface {
	MakeVolumeId() string
	ParseRequest(req *csi.CreateVolumeRequest, parser *serialmaps.Parser) error
	InitHandles() error
	Create(ctx context.Context) error
	GetVolumeContext() volumecontext.VolumeContext
	GetSize() int64
	SetSize(ctx context.Context, newSize int64) error
	Publish(ctx context.Context, nodeName string) (publishcontext.PublishContext, error)
	// Publish(ctx context.Context, req fileshare.PublishRequest) (publishcontext.PublishContext, error) 
	Unpublish(ctx context.Context, nodeId string) error
	UnpublishAll(ctx context.Context) error
	Destroy(ctx context.Context) error
	Save(ctx context.Context) error
}

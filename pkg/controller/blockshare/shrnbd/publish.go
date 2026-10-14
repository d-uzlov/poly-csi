package shrnbd

import (
	"context"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/controller/blockshare"
)

func (sh *share) Publish(ctx context.Context, req blockshare.PublishRequest) (*blockshare.PublishContext, error) {
	return &blockshare.PublishContext{
		Protocol: blockapi.NbdShare{
			ExportName: sh.ExportName,
			Port:       sh.ConnectPort,
		},
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized string) error {
	return nil
}

func (sh *share) UnpublishAll(ctx context.Context) error {
	return nil
}

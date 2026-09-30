package shrnbd

import (
	"context"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/protocols"
)

func (sh *share) Publish(ctx context.Context, req shares.PublishRequest) (*shares.PublishContext, error) {
	return &shares.PublishContext{
		Context: protocols.NbdShare{
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

package shrsamba

import (
	"context"

	"meoe.io/poly-csi/pkg/api/fsapi"
	"meoe.io/poly-csi/pkg/controller/fileshare"
)

func (sh *share) Publish(ctx context.Context, req fileshare.PublishRequest) (*fileshare.PublishContext, error) {
	return &fileshare.PublishContext{
		Protocol: fsapi.SmbShare{
			ShareName:    sh.ShareName,
			UserName:     sh.UserName,
			UserPassword: sh.UserPass,
		},
		LocalData: nil,
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized string) error {
	return nil
}

func (sh *share) UnpublishAll(ctx context.Context) error {
	return nil
}

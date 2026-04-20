package shrsamba

import (
	"context"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrcifs"
)

func (sh *share) Publish(ctx context.Context, mi connectors.ConnectorNodeInfo) (*shares.PublishContext, error) {
	return &shares.PublishContext{
		Public: cntrcifs.PublicContext{
			ShareName: sh.ShareName,
			UserName:  sh.UserName,
		},
		Secret: cntrcifs.SecretContext{
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

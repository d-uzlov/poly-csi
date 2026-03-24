package shrsamba

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrcifs"
)

func (sh *share) Publish(ctx context.Context, shareData string, mi connectors.ConnectorNodeInfo) (*shares.PublishContext, error) {
	c, err := parseShareData(shareData)
	if err != nil {
		return nil, fmt.Errorf("parse share data: %w", err)
	}

	return &shares.PublishContext{
		Public: cntrcifs.PublicContext{
			ShareName: c.ShareName,
			UserName:  c.UserName,
		},
		Secret: cntrcifs.SecretContext{
			UserPassword: c.UserPass,
		},
		LocalData: nil,
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, shareData string, serialized string) error {
	return nil
}

func (sh *share) UnpublishAll(ctx context.Context, shareData string) error {
	return nil
}

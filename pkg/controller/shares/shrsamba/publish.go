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

	result := &shares.PublishContext{
		Public: map[string]string{},
		Secret: map[string]string{},
	}
	cc := cntrcifs.CifsMountContext{
		ShareName:    c.ShareName,
		UserName:     c.UserName,
		UserPassword: c.UserPass,
	}
	err = cc.InjectPublic(result.Public)
	if err != nil {
		return nil, fmt.Errorf("inject public values into context: %w", err)
	}
	err = cc.InjectSecret(result.Secret)
	if err != nil {
		return nil, fmt.Errorf("inject secret values into context: %w", err)
	}
	return result, nil
}

func (sh *share) Unpublish(ctx context.Context, shareData string, serialized string) error {
	return nil
}

func (sh *share) UnpublishAll(ctx context.Context, shareData string) error {
	return nil
}

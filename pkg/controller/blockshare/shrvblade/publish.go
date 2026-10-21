package shrvblade

import (
	"context"
	"encoding/json"

	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/controller/blockshare"
)

func (s *share) Publish(ctx context.Context, req blockshare.PublishRequest) (*blockshare.PublishContext, error) {
	return &blockshare.PublishContext{
		Protocol: blockspec.AoeShare{
			Shelf: new(int(s.Shelf)),
			Slot:  new(int(s.Slot)),
		},
	}, nil
}

func (s *share) Unpublish(ctx context.Context, serialized json.RawMessage) error {
	return nil
}

func (s *share) UnpublishAll(ctx context.Context) error {
	return nil
}

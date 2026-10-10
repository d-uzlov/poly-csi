package shrvblade

import (
	"context"

	"meoe.io/poly-csi/pkg/api/protocols"
	"meoe.io/poly-csi/pkg/controller/shares"
)

func (s *share) Publish(ctx context.Context, req shares.PublishRequest) (*shares.PublishContext, error) {
	return &shares.PublishContext{
		Protocol: protocols.AoeShare{
			Shelf: new(int(s.Shelf)),
			Slot:  new(int(s.Slot)),
		},
	}, nil
}

func (s *share) Unpublish(ctx context.Context, serialized string) error {
	return nil
}

func (s *share) UnpublishAll(ctx context.Context) error {
	return nil
}

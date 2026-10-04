package shrvblade

import (
	"context"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/protocols"
)

func (s *share) Publish(ctx context.Context, req shares.PublishRequest) (*shares.PublishContext, error) {
	return &shares.PublishContext{
		Context: protocols.AoeShare{
			Shelf: s.Shelf,
			Slot:  s.Slot,
		},
	}, nil
}

func (s *share) Unpublish(ctx context.Context, serialized string) error {
	return nil
}

func (s *share) UnpublishAll(ctx context.Context) error {
	return nil
}

package nvmet

import (
	"context"

	"meoe.io/poly-csi/pkg/transport"
)

type Snippets struct {
	// TODO move context to methods?
	ctx     context.Context
	tr      transport.Transport
	useSudo bool
}

func NewSnippets(ctx context.Context, tr transport.Transport, useSudo bool) Snippets {
	return Snippets{
		ctx:     ctx,
		tr:      tr,
		useSudo: useSudo,
	}
}

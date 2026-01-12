package nvmet

import (
	"context"
)

type Snippets struct {
	ctx     context.Context
	useSudo bool
}

func NewSnippets(ctx context.Context, useSudo bool) Snippets {
	return Snippets{
		ctx:     ctx,
		useSudo: useSudo,
	}
}

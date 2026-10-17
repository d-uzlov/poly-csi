package dispatcher

import (
	"context"
)

type contextKey struct{}

func NewCtx(parent context.Context, instance *DispatcherConf) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	return context.WithValue(parent, contextKey{}, instance)
}

func FromCtx(ctx context.Context) *DispatcherConf {
	v := ctx.Value(contextKey{})
	if v == nil {
		panic("missing config in context")
	}

	switch v := v.(type) {
	case *DispatcherConf:
		return v
	default:
		panic("unexpected object type in context at nodedispatcher.contextKey")
	}
}

package nodedispatcher

import (
	"context"

	"meoe.io/poly-csi/pkg/node/nodeconfig"
)

type contextKey struct{}

func NewCtx(parent context.Context, instance *nodeconfig.ParsedConfig) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	return context.WithValue(parent, contextKey{}, instance)
}

func FromCtx(ctx context.Context) *nodeconfig.ParsedConfig {
	v := ctx.Value(contextKey{})
	if v == nil {
		panic("missing config in context")
	}

	switch v := v.(type) {
	case *nodeconfig.ParsedConfig:
		return v
	default:
		panic("unexpected object type in context at nodedispatcher.contextKey")
	}
}

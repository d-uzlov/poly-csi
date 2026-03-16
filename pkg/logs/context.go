package logs

import (
	"context"
)

type contextKey struct{}

func NewCtx(parent context.Context, instance *Instance) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	return context.WithValue(parent, contextKey{}, instance)
}

// FromCtx extracts logs.Instance injected via logs.NewCtx.
// If ctx does not contain injected value, default value is returned.
func FromCtx(ctx context.Context) *Instance {
	if ctx == nil {
		return DefaultInstance()
	}

	v := ctx.Value(contextKey{})
	if v == nil {
		return DefaultInstance()
	}

	switch v := v.(type) {
	case *Instance:
		return v
	default:
		panic("unexpected object type in context at logs.contextKey")
	}
}

package uuidctx

import "context"

type contextKey struct{}

func NewCtx(parent context.Context, value string) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	return context.WithValue(parent, contextKey{}, value)
}

// FromCtx extracts logs.Instance injected via logs.NewCtx.
// If ctx does not contain injected value, default value is returned.
func FromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	v := ctx.Value(contextKey{})
	if v == nil {
		return ""
	}

	switch v := v.(type) {
	case string:
		return v
	default:
		panic("unexpected object type in context at uuidctx.contextKey")
	}
}

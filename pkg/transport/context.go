package transport

import "context"

type transportKeyType struct{}

var transportKey = transportKeyType{}

func MakeContext(ctx context.Context, tr Transport) context.Context {
	return context.WithValue(ctx, transportKey, tr)
}

func FromContext(ctx context.Context) Transport {
	val := ctx.Value(transportKey)
	if val == nil {
		return nil
	}
	tr, ok := val.(Transport)
	if !ok {
		panic("context is missing transport")
	}
	if tr == nil {
		panic("context is missing transport")
	}
	return tr
}

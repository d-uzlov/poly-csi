package simplerpc

import (
	"context"
	"net/http"
)

type Middleware func(ctx context.Context, r *http.Request, data any, next func(context.Context) any) error

type Opts struct {
	mw Middleware
}

type Option func(o *Opts)

func WithMiddleware(m Middleware) Option {
	return func(o *Opts) {
		o.mw = m
	}
}

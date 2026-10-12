package serialmaps

import "meoe.io/poly-csi/pkg/utils/typeregistry"

type options struct {
	removeToken bool
	reg typeregistry.Registry
}

type Option func(o *options)

func WithRemove(val bool) Option {
	return func(o *options) {
		o.removeToken = val
	}
}

func WithRegistry(reg typeregistry.Registry) Option {
	return func(o *options) {
		o.reg = reg
	}
}

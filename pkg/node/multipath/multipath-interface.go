package multipath

import "context"

type Multipath interface {
	Ready(ctx context.Context) error
	Setup(ctx context.Context, devices []string) (serializedData string, err error)
	GetDevice(ctx context.Context, serializedData string) (device string, err error)
	Destroy(ctx context.Context, serializedData string) error
	Expand(ctx context.Context, serializedData string) error
}

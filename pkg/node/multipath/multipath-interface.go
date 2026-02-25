package multipath

import "context"

type Multipath interface {
	Ready(ctx context.Context) bool
	Setup(ctx context.Context, devices []string) (*MultipathContext, error)
	Destroy(ctx context.Context, serializedData string) error
}

type MultipathContext struct {
	Device         string `json:"device"`
	SerializedData string `json:"serialized"`
}

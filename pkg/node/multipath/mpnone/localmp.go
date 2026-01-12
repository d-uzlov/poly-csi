package mpnone

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/node/multipath"
)

type localMp struct {
}

func NewNoneMp() multipath.Multipath {
	return &localMp{}
}

func (l *localMp) Ready(ctx context.Context) bool {
	return false
}

func (l *localMp) Setup(ctx context.Context, devices []string) (*multipath.MultipathContext, error) {
	return nil, fmt.Errorf("multipath is disabled")
}

func (l *localMp) Destroy(ctx context.Context, mc multipath.MultipathContext) error {
	return fmt.Errorf("multipath is disabled")
}

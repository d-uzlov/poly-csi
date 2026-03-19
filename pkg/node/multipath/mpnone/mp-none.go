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

func (l *localMp) Ready(ctx context.Context) error {
	return fmt.Errorf("multipath stub")
}

func (l *localMp) Setup(ctx context.Context, devices []string) (serializedData string, err error) {
	return "", fmt.Errorf("multipath stub")
}

func (l *localMp) GetDevice(ctx context.Context, serializedData string) (device string, err error) {
	return "", fmt.Errorf("multipath stub")
}

func (l *localMp) Destroy(ctx context.Context, serializedData string) error {
	return nil
}

func (l *localMp) Expand(ctx context.Context, serializedData string) error {
	return nil
}

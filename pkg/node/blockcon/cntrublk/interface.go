package cntrublk

import (
	"context"

	"meoe.io/poly-csi/pkg/node/blockcon"
	"meoe.io/poly-csi/pkg/utils/ublk"
)

type Supervisor interface {
	StartupRestore() error
	Launch(ctx context.Context, uuid string, cp ublk.CreateParams, ep ublk.ExtraParams) (result ublk.DeviceInfo, err error)
	Remove(uuid string) error
}

type InflateFunc func(sv Supervisor, conf blockcon.CommonConfig) blockcon.Manager

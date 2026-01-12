package nodeinfo

import (
	"context"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/noderegistry/nodremotereg"
	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
)

func SetupPush(ctx context.Context, address string, interval time.Duration, getInfo func() regnodeinfo.NodeInfo, doneCallback func()) error {

	go func() {
		defer doneCallback()
		reg := nodremotereg.NewNodeRemoteRegistry(address)

		for ctx.Err() == nil {
			nodeInfo := getInfo()
			err := reg.Set(nodeInfo)
			if err != nil {
				slogctx.FromCtx(ctx).Error("failed to push node info to registry", "error", err)
			}

			select {
			case <-ctx.Done():
			case <-time.Tick(interval):
			}
		}
	}()

	return nil
}

package nodeinfo

import (
	"context"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/noderegistry/nodremotereg"
	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
)

func SetupPush(ctx context.Context, address string, interval time.Duration, getInfo func() (regnodeinfo.RegNodeInfo, error), doneCallback func()) error {

	go func() {
		defer doneCallback()
		reg := nodremotereg.NewNodeRemoteRegistry(address)

		slogctx.FromCtx(ctx).Info("starting node info push")
		for ctx.Err() == nil {
			nodeInfo, err := getInfo()
			if err != nil {
				slogctx.FromCtx(ctx).Error("failed to gather node info", "error", err)
			} else {
				err = reg.Set(nodeInfo)
				if err != nil {
					slogctx.FromCtx(ctx).Error("failed to push node info to registry", "error", err)
				} else {
					slogctx.FromCtx(ctx).Debug("successfully pushed node info")
				}
			}

			select {
			case <-ctx.Done():
			case <-time.Tick(interval):
			}
		}
		slogctx.FromCtx(ctx).Info("stopped node info push")
	}()

	return nil
}

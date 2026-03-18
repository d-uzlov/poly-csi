package nodeinfo

import (
	"context"
	"time"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/noderegistry/nodremotereg"
	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
)

func PeriodicInfoPush(ctx context.Context, logConfig logs.Config, address string, interval time.Duration, getInfo func(ctx context.Context) (regnodeinfo.RegNodeInfo, error), doneCallback func()) {
	defer doneCallback()
	reg := nodremotereg.NewNodeRemoteRegistry(address)

	startLogger := logConfig.MakeInstance()

	// skipOnSuccess should not apply to this logger
	startLogger.Commit(false)

	startLogger.MicroOp("starting node info push")
	for ctx.Err() == nil {
		pushInfo(ctx, logConfig, reg, getInfo)

		select {
		case <-ctx.Done():
		case <-time.Tick(interval):
		}
	}
	startLogger.MicroOp("stopped node info push")
}

func pushInfo(ctx context.Context, logConfig logs.Config, reg nodremotereg.NodeRemoveRegistry, getInfo func(ctx context.Context) (regnodeinfo.RegNodeInfo, error)) {
	logger := logConfig.MakeInstance()
	ctx = logs.NewCtx(ctx, logger)

	nodeInfo, err := getInfo(ctx)
	if err != nil {
		logger.Error("failed to gather node info", "error", err.Error())
		logger.Commit(false)
		return
	}

	err = reg.Set(nodeInfo)
	if err != nil {
		logger.Error("failed to push node info to registry", "error", err.Error())
		logger.Commit(false)
		return
	}

	logger.MicroOp("successfully pushed node info")
	logger.Commit(true)
}

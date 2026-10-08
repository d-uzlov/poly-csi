package nodeinfo

import (
	"context"
	"time"

	"meoe.io/poly-csi/pkg/api/node_registry"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/noderegistry/noderegclient"
)

func PeriodicInfoPush(ctx context.Context, logConfig logs.Config, client noderegclient.Client, interval time.Duration, getInfo func(ctx context.Context) (*NodeInfo, error), doneCallback func()) {
	defer doneCallback()

	startLogger := logConfig.MakeInstance()

	// skipOnSuccess should not apply to this logger
	startLogger.Commit(false)

	startLogger.MicroOp("starting node info push")
	for ctx.Err() == nil {
		pushInfo(ctx, logConfig, client, getInfo)

		select {
		case <-ctx.Done():
		case <-time.Tick(interval):
		}
	}
	startLogger.MicroOp("stopped node info push")
}

func pushInfo(ctx context.Context, logConfig logs.Config, reg noderegclient.Client, getInfo func(ctx context.Context) (*NodeInfo, error)) {
	logger := logConfig.MakeInstance()
	ctx = logs.NewCtx(ctx, logger)

	nodeInfo, err := getInfo(ctx)
	if err != nil {
		logger.Error("failed to gather node info", "error", err.Error())
		logger.Commit(false)
		return
	}

	serialized, err := nodeInfo.Serialize()
	if err != nil {
		logger.Error("failed to serialize node info", "error", err.Error())
		logger.Commit(false)
		return
	}

	err = reg.Register(ctx, node_registry.RegisterRequest{
		ID:   nodeInfo.Id,
		Info: serialized,
	})
	if err != nil {
		logger.Error("failed to push node info to registry", "error", err.Error())
		logger.Commit(false)
		return
	}

	logger.MicroOp("successfully pushed node info")
	logger.Commit(true)
}

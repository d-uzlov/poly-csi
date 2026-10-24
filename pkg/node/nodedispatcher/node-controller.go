package nodedispatcher

import (
	"context"
	"log/slog"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"

	"meoe.io/poly-csi/pkg/node/nodeconfig"
	"meoe.io/poly-csi/pkg/node/nodeinfo"
	"meoe.io/poly-csi/pkg/noderegistry/noderegclient"
)

func New(logger *slog.Logger, name string, version string, nodeId string) (*Node, error) {
	return &Node{
		name:    name,
		version: version,
		nodeId:  nodeId,
	}, nil
}

type Node struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedNodeServer

	name    string
	version string
	nodeId  string

	confMutex sync.RWMutex
	conf      *nodeconfig.ParsedConfig

	nodeInfoCancel func(ctx context.Context)
}

func (n *Node) ReloadConfig(ctx context.Context, newConf *nodeconfig.ParsedConfig) error {
	n.confMutex.Lock()
	defer n.confMutex.Unlock()

	if n.nodeInfoCancel != nil {
		slogctx.FromCtx(ctx).Debug("cancelling node info push")
		n.nodeInfoCancel(ctx)
		n.nodeInfoCancel = nil
	}

	pushCtx, pushCancel := context.WithCancel(ctx)
	pushCh := make(chan struct{})

	presetNames := []string{}
	for k := range newConf.BlockCgroupPresets {
		presetNames = append(presetNames, k)
	}

	registryClient := noderegclient.New(newConf.NodeInfoDb.Address)

	slogctx.FromCtx(ctx).Debug("starting node info push", "presets", presetNames)
	go nodeinfo.PeriodicInfoPush(pushCtx, newConf.GetNodeInfoLogConfig(), registryClient, newConf.NodeInfoDb.Interval,
		func(ctx context.Context) (*nodeinfo.NodeInfo, error) {
			return nodeinfo.GatherNodeInfo(ctx, n.nodeId, newConf.Encrypt.NodeKey, newConf.BlockConnectors, newConf.Networks, presetNames)
		}, func() {
			close(pushCh)
		},
	)

	n.nodeInfoCancel = func(ctx context.Context) {
		pushCancel()
		select {
		case <-pushCh:
		case <-ctx.Done():
		}
	}

	n.conf = newConf

	return nil
}

func (n *Node) GetConfig() (conf *nodeconfig.ParsedConfig) {
	n.confMutex.RLock()
	defer n.confMutex.RUnlock()

	result := n.conf
	return result
}

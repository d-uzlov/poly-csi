package nodedispatcher

import (
	"context"
	"log/slog"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/nodeconfig"
	"meoe.io/poly-csi/pkg/node/nodeinfo"
	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
	"meoe.io/poly-csi/pkg/transport/localexec"
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
	tr := localexec.New(newConf.ExecConfig)

	mounts := map[string]connectors.Connector{}
	for _, v := range newConf.Connectors {
		if !v.Valid {
			continue
		}
		m := v.Inflate()
		if m == nil {
			continue
		}
		opaqueConfig, err := yaml.Marshal(v.Conf.Config)
		if err != nil {
			slogctx.FromCtx(ctx).Error("failed to initialize connector[id=%s] during reload: marshal config: %w", v.Conf.Id, err)
			continue
		}
		err = m.Configure(ctx, connectors.CommonConfig{
			Tr:        tr,
			DevFolder: newConf.DevFolder,
			Preset:    map[string]string{},
		}, opaqueConfig)
		if err != nil {
			slogctx.FromCtx(ctx).Error("failed to initialize connector[id=%s] during reload: %w", v.Conf.Id, err)
			continue
		}
		mounts[v.Conf.Id] = m
	}

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
	for k := range newConf.Presets {
		presetNames = append(presetNames, k)
	}

	slogctx.FromCtx(ctx).Debug("starting node info push", "presets", presetNames)
	go nodeinfo.PeriodicInfoPush(pushCtx, newConf.GetNodeInfoLogConfig(), newConf.NodeInfoDb.Address, newConf.NodeInfoDb.Interval,
		func(ctx context.Context) (regnodeinfo.RegNodeInfo, error) {
			return nodeinfo.GatherNodeInfo(ctx, n.nodeId, newConf.Encrypt.NodeKey, mounts, newConf.Networks, presetNames)
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

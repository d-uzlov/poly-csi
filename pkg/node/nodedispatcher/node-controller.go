package nodedispatcher

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/node/nodeinfo"
	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/encrypt"
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
	conf      parsedConfig
	tr        transport.Transport

	nodeInfoCancel func()
	nodeInfoCh     <-chan struct{}
}

func (n *Node) ReloadConfig(ctx context.Context, newConf parsedConfig) error {
	tr, err := localexec.New(newConf.ExecConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec object: %w", err)
	}

	mounts := map[string]connectors.Connector{}
	for _, v := range newConf.Mounts {
		if !v.Valid {
			continue
		}
		m := v.Inflate(tr)
		if m == nil {
			continue
		}
		mounts[v.Conf.Id] = m
	}
	var mp multipath.Multipath
	if newConf.Multipath.Valid {
		mp = newConf.Multipath.Inflate(tr)
	} else {
		slogctx.FromCtx(ctx).Error("multipath config is not valid, disabling multipath")
	}

	aesSecret := make([]byte, 32)
	// TODO move this to startup
	_, err = rand.Read(aesSecret)
	if err != nil {
		return fmt.Errorf("generate random bytes for aes: %w", err)
	}

	n.confMutex.Lock()
	defer n.confMutex.Unlock()

	if n.nodeInfoCh != nil && n.nodeInfoCancel != nil {
		slogctx.FromCtx(ctx).Debug("cancelling node info push")
		n.nodeInfoCancel()
		<-n.nodeInfoCh
		n.nodeInfoCancel = nil
		n.nodeInfoCh = nil
	}

	pushCtx, pushCancel := context.WithCancel(ctx)
	pushCh := make(chan struct{})

	go nodeinfo.PeriodicInfoPush(pushCtx, n.conf.getNodeInfoLogConfig(), newConf.NodeInfoDb.Address, newConf.NodeInfoDb.Interval,
		func(ctx context.Context) (regnodeinfo.RegNodeInfo, error) {
			return nodeinfo.GatherNodeInfo(ctx, n.nodeId, newConf.Encrypt.NodeKey, mounts, aesSecret, newConf.Encrypt.ControllerKey, mp, newConf.Networks)
		}, func() {
			pushCh <- struct{}{}
		},
	)

	n.nodeInfoCancel = pushCancel
	n.nodeInfoCh = pushCh

	n.conf = newConf
	if n.tr != nil {
		// TODO this is unsafe, we need to use extra RW-lock for n.tr
		n.tr.Cleanup()
	}
	n.tr = tr

	return nil
}

func (n *Node) findConnectorById(id string) connectors.Connector {
	n.confMutex.RLock()
	defer n.confMutex.RUnlock()

	res := n.conf.findConnectorById(id)
	if res == nil {
		return nil
	}
	return res.Inflate(n.tr)
}

func (n *Node) getMultipath() multipath.Multipath {
	n.confMutex.RLock()
	defer n.confMutex.RUnlock()

	if !n.conf.Multipath.Valid {
		return nil
	}
	if n.conf.Multipath.Inflate == nil {
		return nil
	}
	return n.conf.Multipath.Inflate(n.tr)
}

func (n *Node) getPrivateKey() encrypt.RsaPrivateKey {
	n.confMutex.RLock()
	defer n.confMutex.RUnlock()

	return n.conf.Encrypt.NodeKey
}

func (n *Node) getTransport() transport.Transport {
	n.confMutex.RLock()
	defer n.confMutex.RUnlock()

	return n.tr
}

func (n *Node) GetRpcLogConfig(rpcPath string) logs.Config {
	n.confMutex.RLock()
	defer n.confMutex.RUnlock()

	return n.conf.getRpcLogConfig(rpcPath)
}

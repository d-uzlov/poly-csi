package nodedispatcher

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/node/nodeinfo"
	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

func CreateNode(logger *slog.Logger, name string, version string, nodeId string) (*Node, error) {
	tr, err := localexec.New()
	if err != nil {
		panic(err.Error())
	}

	return &Node{
		name:    name,
		version: version,
		nodeId:  nodeId,
		tr:      tr,
	}, nil
}

type Node struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedNodeServer

	name    string
	version string
	nodeId  string
	tr      transport.Transport

	confMutex sync.RWMutex
	conf      parsedConfig

	nodeInfoCancel func()
	nodeInfoCh     <-chan struct{}
}

func (n *Node) ReloadConfig(ctx context.Context, newConf parsedConfig) error {
	if n.nodeInfoCh != nil && n.nodeInfoCancel != nil {
		n.nodeInfoCancel()
		<-n.nodeInfoCh
		n.nodeInfoCancel = nil
		n.nodeInfoCh = nil
	}

	mounts := map[string]connectors.Connector{}
	for _, v := range newConf.Mounts {
		if !v.Valid {
			continue
		}
		m := v.Inflate(n.tr)
		if m == nil {
			continue
		}
		mounts[v.Conf.Id] = m
	}
	var mp multipath.Multipath
	if newConf.Multipath.Valid {
		mp = newConf.Multipath.Inflate(n.tr)
	} else {
		slogctx.FromCtx(ctx).Error("multipath config is not valid, disabling multipath")
	}

	aesSecret := make([]byte, 32)
	_, err := rand.Read(aesSecret)
	if err != nil {
		return fmt.Errorf("generate random bytes for aes: %w", err)
	}

	pushCtx, pushCancel := context.WithCancel(ctx)
	pushCh := make(chan struct{})

	nodeinfo.SetupPush(pushCtx, newConf.NodeInfoDb.Address, newConf.NodeInfoDb.Interval, func() (regnodeinfo.RegNodeInfo, error) {
		return nodeinfo.GatherNodeInfo(pushCtx, n.nodeId, newConf.Encrypt.NodeKey, mounts, aesSecret, newConf.Encrypt.ControllerKey, mp, newConf.Networks)
	}, func() {})

	n.nodeInfoCancel = pushCancel
	n.nodeInfoCh = pushCh

	n.confMutex.Lock()
	defer n.confMutex.Unlock()

	n.conf = newConf
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

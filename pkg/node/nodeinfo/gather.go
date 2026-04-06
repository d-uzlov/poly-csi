package nodeinfo

import (
	"context"
	"fmt"
	"net"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/nodeconfig"
	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

func GatherNodeInfo(ctx context.Context, nodeId string, pk encrypt.RsaPrivateKey, connectorList map[string]connectors.Connector, networks []nodeconfig.NodeNetworkConfig) (regnodeinfo.RegNodeInfo, error) {
	nodePublicKey := pk.GetPublicKey()
	info := NodeInfo{
		Id:         nodeId,
		PublicKey:  nodePublicKey,
		Connectors: map[string]connectors.ConnectorNodeInfo{},
	}

	for k, v := range connectorList {
		mi, err := v.GetInfo(ctx)
		if err != nil {
			slogctx.FromCtx(ctx).Error("connector.GetInfo failed", "error", err.Error())
			continue
		}
		info.Connectors[k] = *mi
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return regnodeinfo.RegNodeInfo{}, fmt.Errorf("list network interfaces: %w", err)
	}

	var addresses []string
	for _, iface := range interfaces {
		al, err := iface.Addrs()
		if err != nil {
			return regnodeinfo.RegNodeInfo{}, fmt.Errorf("list unicast addresses for interface %s: %w", iface.Name, err)
		}
		for _, a := range al {
			addr, ok := a.(*net.IPNet)
			if !ok {
				return regnodeinfo.RegNodeInfo{}, fmt.Errorf("cast addresses %v to net.IPNet: %w", a, err)
			}
			for _, n := range networks {
				matchesInclude := false
				matchesExclude := false
				for _, v := range n.Include {
					// TODO move CIDR parsing to config reloading
					_, cidr, err := net.ParseCIDR(v)
					if err != nil {
						return regnodeinfo.RegNodeInfo{}, fmt.Errorf("parse cidr %s from include: %w", v, err)
					}
					if cidr.Contains(addr.IP) {
						matchesInclude = true
						break
					}
				}
				for _, v := range n.Exclude {
					_, cidr, err := net.ParseCIDR(v)
					if err != nil {
						return regnodeinfo.RegNodeInfo{}, fmt.Errorf("parse cidr %s from exclude: %w", v, err)
					}
					if cidr.Contains(addr.IP) {
						matchesExclude = true
						break
					}
				}
				if matchesInclude && !matchesExclude {
					addresses = append(addresses, addr.IP.String())
				}
			}
		}
	}
	info.Ips = addresses

	err = info.Validate()
	if err != nil {
		return regnodeinfo.RegNodeInfo{}, fmt.Errorf("validate: %w", err)
	}

	serialized, err := info.Serialize()
	if err != nil {
		return regnodeinfo.RegNodeInfo{}, fmt.Errorf("serialize: %w", err)
	}

	return regnodeinfo.RegNodeInfo{
		Id:   nodeId,
		Data: serialized,
	}, nil
}

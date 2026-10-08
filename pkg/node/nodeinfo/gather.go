package nodeinfo

import (
	"context"
	"fmt"
	"net"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/nodeconfig"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

func GatherNodeInfo(ctx context.Context, nodeId string, pk encrypt.RsaPrivateKey, connectorList map[string]connectors.Manager, networks []nodeconfig.ParsedNetworks, presets []string) (*NodeInfo, error) {
	nodePublicKey := pk.GetPublicKey()
	info := NodeInfo{
		Id:         nodeId,
		PublicKey:  nodePublicKey,
		Connectors: map[string]ConnectorInfo{},
		Presets:    presets,
	}

	for k, v := range connectorList {
		caps, err := v.Capabilities(ctx)
		if err != nil {
			slogctx.FromCtx(ctx).Error("connector.Capabilities failed", "error", err.Error())
			continue
		}
		mi, err := v.GetInfo(ctx)
		if err != nil {
			// TODO transient errors are logged, but there are no confirmations when they are cleared
			slogctx.FromCtx(ctx).Error("connector.GetInfo failed", "error", err.Error())
			continue
		}
		info.Connectors[k] = ConnectorInfo{
			Protocol: caps.Protocol,
			Info:     mi,
		}
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}

	var addresses []string
	for _, iface := range interfaces {
		al, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("list unicast addresses for interface %s: %w", iface.Name, err)
		}
		for _, a := range al {
			addr, ok := a.(*net.IPNet)
			if !ok {
				return nil, fmt.Errorf("cast addresses %v to net.IPNet: %w", a, err)
			}
			for _, n := range networks {
				matchesInclude := false
				matchesExclude := false
				for _, v := range n.Include {
					if v.Contains(addr.IP) {
						matchesInclude = true
						break
					}
				}
				for _, v := range n.Exclude {
					if v.Contains(addr.IP) {
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
		return nil, fmt.Errorf("validate: %w", err)
	}

	return &info, nil
}

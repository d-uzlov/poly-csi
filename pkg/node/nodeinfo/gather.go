package nodeinfo

import (
	"context"
	"net"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/node/nodeconfig"
	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

func GatherNodeInfo(ctx context.Context, nodeId string, pk encrypt.RsaPrivateKey, mounters []mounts.NodeMounter, aesSecret []byte, remotePubKey encrypt.RsaPublicKey, mp multipath.Multipath, networks []nodeconfig.NodeNetworkConfig) regnodeinfo.NodeInfo {
	nodePublicKey := pk.GetPublicKey()
	info := NodeInfo{
		Id:        nodeId,
		PublicKey: &nodePublicKey,
	}

	if mp != nil {
		info.HasMultipath = mp.Ready(ctx)
	}

	for _, v := range mounters {
		mi, err := v.GetInfo(ctx)
		if err != nil {
			slogctx.FromCtx(ctx).Error("mounter.GetInfo failed", "error", err)
			continue
		}
		info.Mounters = append(info.Mounters, *mi)
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		slogctx.FromCtx(ctx).Error("failed to list network interfaces", "error", err)
		return regnodeinfo.NodeInfo{}
	}

	var addresses []string
	for _, iface := range interfaces {
		al, err := iface.Addrs()
		if err != nil {
			slogctx.FromCtx(ctx).Error("failed to list unicast addresses for interface", "error", err, "interface", iface.Name)
			return regnodeinfo.NodeInfo{}
		}
		for _, a := range al {
			addr, ok := a.(*net.IPNet)
			if !ok {
				slogctx.FromCtx(ctx).Error("failed to cast addresses to net.IPNet", "error", err, "addr", a)
				return regnodeinfo.NodeInfo{}
			}
			for _, n := range networks {
				matchesInclude := false
				matchesExclude := false
				for _, v := range n.Include {
					_, cidr, err := net.ParseCIDR(v)
					if err != nil {
						slogctx.FromCtx(ctx).Error("failed to parse cidr", "error", err, "cidr", v)
						return regnodeinfo.NodeInfo{}
					}
					if cidr.Contains(addr.IP) {
						matchesInclude = true
						break
					}
				}
				for _, v := range n.Exclude {
					_, cidr, err := net.ParseCIDR(v)
					if err != nil {
						slogctx.FromCtx(ctx).Error("failed to parse cidr", "error", err, "cidr", v)
						return regnodeinfo.NodeInfo{}
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

	if info.Validate() != nil {
		slogctx.FromCtx(ctx).Error("failed to validate node info", "error", err)
		return regnodeinfo.NodeInfo{}
	}

	serialized, err := info.Serialize(remotePubKey, aesSecret)
	if err != nil {
		slogctx.FromCtx(ctx).Error("failed to serialize node info", "error", err)
		return regnodeinfo.NodeInfo{}
	}

	return regnodeinfo.NodeInfo{
		Id:   nodeId,
		Data: serialized,
	}
}

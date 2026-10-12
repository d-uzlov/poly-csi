package nodeinfo

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/node_registry"
)

type NodeInfo struct {
	Id         string                   `json:"id"`
	PublicKey  []byte                   `json:"public_key"`
	Connectors map[string]ConnectorInfo `json:"connectors"`
	Ips        []string                 `json:"ips"`
	Presets    []string                 `json:"presets"`
}

type ConnectorInfo struct {
	Protocol blockapi.Name     `json:"protocol"`
	Info     blockapi.NodeInfo `json:"info"`
}

func (ni *NodeInfo) Validate() error {
	if ni.Id == "" {
		return fmt.Errorf("missing node id")
	}
	if ni.PublicKey == nil {
		return fmt.Errorf("missing public key")
	}
	return nil
}

func (ni *NodeInfo) Serialize() (node_registry.NodeInfo, error) {
	result := node_registry.NodeInfo{
		IPS:        ni.Ips,
		Presets:    ni.Presets,
		PublicKey:  base64.StdEncoding.EncodeToString(ni.PublicKey),
		Connectors: map[string]node_registry.ConnectorInfo{},
	}
	for k, v := range ni.Connectors {
		data, err := json.Marshal(v.Info)
		if err != nil {
			return result, fmt.Errorf("connector '%s': marshal protocol '%s' info %v: %w", k, v.Protocol, v.Info, err)
		}
		result.Connectors[k] = node_registry.ConnectorInfo{
			Protocol: string(v.Protocol),
			Data:     string(data),
		}
	}

	return result, nil
}

func ParseNodeInfo(id string, src node_registry.NodeInfo) (*NodeInfo, error) {
	result := NodeInfo{
		Id:         id,
		Ips:        src.IPS,
		Presets:    src.Presets,
		Connectors: map[string]ConnectorInfo{},
	}
	publicKey, err := base64.StdEncoding.DecodeString(src.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("public key: %w", err)
	}
	result.PublicKey = publicKey

	for k, v := range src.Connectors {
		ci := ConnectorInfo{
			Protocol: blockapi.Name(v.Protocol),
		}
		err := blockapi.ParseNodeInfo(blockapi.Name(v.Protocol), v.Data, &ci.Info)
		if err != nil {
			return nil, fmt.Errorf("parse connector '%s' protocol '%s' info: %w", k, v.Protocol, err)
		}
		result.Connectors[k] = ci
	}

	return &result, nil
}

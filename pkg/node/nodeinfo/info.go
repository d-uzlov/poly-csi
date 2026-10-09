package nodeinfo

import (
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/node_registry"
	"meoe.io/poly-csi/pkg/protocols"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

type NodeInfo struct {
	Id         string                   `json:"id"`
	PublicKey  encrypt.RsaPublicKey     `json:"public_key"`
	Connectors map[string]ConnectorInfo `json:"connectors"`
	Ips        []string                 `json:"ips"`
	Presets    []string                 `json:"presets"`
}

type ConnectorInfo struct {
	Protocol protocols.Name     `json:"protocol"`
	Info     protocols.NodeInfo `json:"info"`
}

func (ni *NodeInfo) Validate() error {
	if ni.Id == "" {
		return fmt.Errorf("node id is empty")
	}
	err := ni.PublicKey.Validate()
	if err != nil {
		return fmt.Errorf("public key: %w", err)
	}
	return nil
}

func (ni *NodeInfo) Serialize() (node_registry.NodeInfo, error) {
	result := node_registry.NodeInfo{
		IPS:        ni.Ips,
		Presets:    ni.Presets,
		PublicKey:  string(ni.PublicKey.SerializePem()),
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
	pk, err := encrypt.ParsePublicRsaPem([]byte(src.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("public key: %w", err)
	}
	if pk == nil {
		return nil, fmt.Errorf("node public key: nil")
	}
	result.PublicKey = *pk

	for k, v := range src.Connectors {
		ci := ConnectorInfo{
			Protocol: protocols.Name(v.Protocol),
		}
		err := protocols.ParseNodeInfo(protocols.Name(v.Protocol), v.Data, &ci.Info)
		if err != nil {
			return nil, fmt.Errorf("parse connector '%s' protocol '%s' info: %w", k, v.Protocol, err)
		}
		result.Connectors[k] = ci
	}

	return &result, nil
}

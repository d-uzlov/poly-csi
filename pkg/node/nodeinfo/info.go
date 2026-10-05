package nodeinfo

import (
	"encoding/json"
	"fmt"
	"strings"

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

func (ni *NodeInfo) Serialize() (map[string]string, error) {
	result := map[string]string{
		"id":              ni.Id,
		"node_public_key": string(ni.PublicKey.SerializePem()),
		"has_multipath":   "0",
		"ips":             strings.Join(ni.Ips, ","),
		"presets":         strings.Join(ni.Presets, ","),
	}
	connectorsSerialized, err := json.Marshal(ni.Connectors)
	if err != nil {
		return nil, err
	}
	result["connectors"] = string(connectorsSerialized)

	return result, nil
}

func ParseNodeInfo(src map[string]string) (*NodeInfo, error) {
	result := NodeInfo{
		Connectors: map[string]ConnectorInfo{},
	}
	for k, v := range src {
		switch k {
		case "id":
			result.Id = v
			continue
		case "node_public_key":
			pk, err := encrypt.ParsePublicRsaPem([]byte(v))
			if err != nil {
				return nil, fmt.Errorf("parse node public key: %w", err)
			}
			if pk == nil {
				return nil, fmt.Errorf("parse node public key: nil")
			}
			result.PublicKey = *pk
			continue
		case "ips":
			result.Ips = strings.Split(v, ",")
		case "presets":
			result.Presets = strings.Split(v, ",")
		case "connectors":
			err := json.Unmarshal([]byte(v), &result.Connectors)
			if err != nil {
				return nil, err
			}
		}
		// TODO add logs for ignored fields?
	}
	return &result, nil
}

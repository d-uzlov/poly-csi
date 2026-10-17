package node_registry

import "encoding/json"

type RegisterRequest struct {
	ID   string   `json:"id"`
	Info NodeInfo `json:"info"`
}

type NodeInfo struct {
	Connectors map[string]ConnectorInfo `json:"connectors"`
	IPS        []string                 `json:"ips"`
	Presets    []string                 `json:"presets"`
	PublicKey  []byte                   `json:"publicKey"`
}

type ConnectorInfo struct {
	Protocol string          `json:"protocol"`
	Data     json.RawMessage `json:"data"`
}

type RegisterResponse struct {
}

type GetRequest struct {
	ID string `json:"id"`
}

type GetResponse struct {
	Found bool     `json:"found"`
	Info  NodeInfo `json:"info"`
}

package node_registry

type RegisterRequest struct {
	ID   string   `json:"id"`
	Info NodeInfo `json:"info"`
}

type NodeInfo struct {
	Connectors map[string]ConnectorInfo `json:"connectors"`
	IPS        []string                 `json:"ips"`
	Presets    []string                 `json:"presets"`
	PublicKey  string                   `json:"publicKey"`
}

type ConnectorInfo struct {
	Data     string `json:"data"`
	Protocol string `json:"protocol"`
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

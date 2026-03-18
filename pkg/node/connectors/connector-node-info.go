package connectors

import (
	"encoding/json"
)

type ConnectorNodeInfo struct {
	Public string `json:"public"`
}

func (m ConnectorNodeInfo) Serialize() (string, error) {
	encoded, err := json.Marshal(ConnectorNodeInfo{
		Public: m.Public,
	})
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func DeserializeConnectorNodeInfo(raw string) (ConnectorNodeInfo, error) {
	var result ConnectorNodeInfo
	err := json.Unmarshal([]byte(raw), &result)
	if err != nil {
		return ConnectorNodeInfo{}, err
	}

	return ConnectorNodeInfo{
		Public: result.Public,
	}, nil
}

package regnodeinfo

import (
	"fmt"

	"meoe.io/poly-csi/pkg/node/connectors"
)

type RegNodeInfo struct {
	Id   string            `json:"id"`
	Data map[string]string `json:"data"`
}

func (ni *RegNodeInfo) SetConnectorInfo(id string, info connectors.ConnectorNodeInfo) error {
	if id == "" {
		return fmt.Errorf("connector info: id is empty")
	}
	serialized, err := info.Serialize()
	if err != nil {
		return fmt.Errorf("connector %s: serialize failed: %w", id, err)
	}
	connectorInfoKey := "connector." + id
	ni.Data[connectorInfoKey] = serialized
	return nil
}

func (ni *RegNodeInfo) GetConnectorInfo(connectorId string) (*connectors.ConnectorNodeInfo, error) {
	connectorInfoKey := "connector." + connectorId
	mountInfoRaw := ni.Data[connectorInfoKey]
	if mountInfoRaw == "" {
		return nil, fmt.Errorf("node_info: key '%s' missing", connectorInfoKey)
	}
	mountInfo, err := connectors.DeserializeConnectorNodeInfo(mountInfoRaw)
	if err != nil {
		return nil, err
	}
	return &mountInfo, nil
}

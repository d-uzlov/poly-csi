package cntrnvme

import (
	"context"
	"encoding/json"

	"meoe.io/poly-csi/pkg/node/connectors"
)

type NvmePublicInfo struct {
	HostNqn string `json:"host_nqn"`
	HostId  string `json:"host_id"`
}

func (c *connector) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	info, err := nvmeGetNodeInfo(ctx, c.commonConfig.Tr)
	if err != nil {
		return nil, err
	}

	public, err := json.Marshal(NvmePublicInfo{
		HostNqn: info.hostNqn,
		HostId:  info.hostId,
	})
	if err != nil {
		return nil, err
	}

	return &connectors.ConnectorNodeInfo{
		Public: string(public),
	}, nil
}

func ParseNvmeInfo(src connectors.ConnectorNodeInfo) (NvmePublicInfo, error) {
	var result NvmePublicInfo
	err := json.Unmarshal([]byte(src.Public), &result)
	if err != nil {
		return NvmePublicInfo{}, err
	}
	return result, nil
}

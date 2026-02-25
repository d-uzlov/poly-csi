package mntrnvme

import (
	"context"
	"encoding/json"

	"meoe.io/poly-csi/pkg/node/mounts"
)

type NvmePublicInfo struct {
	HostNqn string `json:"host_nqn"`
	HostId  string `json:"host_id"`
}

func (m *nvmeMounter) GetInfo(ctx context.Context) (*mounts.MounterNodeInfo, error) {
	info, err := nvmeGetNodeInfo(ctx)
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

	return &mounts.MounterNodeInfo{
		Public: string(public),
		Secret: "",
	}, nil
}

func ParseNvmeInfo(src mounts.MounterNodeInfo) (NvmePublicInfo, error) {
	var result NvmePublicInfo
	err := json.Unmarshal([]byte(src.Public), &result)
	if err != nil {
		return NvmePublicInfo{}, err
	}
	return result, nil
}

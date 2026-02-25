package regnodeinfo

import (
	"fmt"

	"meoe.io/poly-csi/pkg/node/mounts"
)

type RegNodeInfo struct {
	Id   string            `json:"id"`
	Data map[string]string `json:"data"`
}

func (ni *RegNodeInfo) SetMounterInfo(id string, info mounts.MounterNodeInfo, aesSecret []byte) error {
	if id == "" {
		return fmt.Errorf("mounter info: id is empty")
	}
	serialized, err := info.Serialize(aesSecret)
	if err != nil {
		return fmt.Errorf("mounter %s: serialize failed: %w", id, err)
	}
	mounterInfoKey := "mounter." + id
	ni.Data[mounterInfoKey] = serialized
	return nil
}

func (ni *RegNodeInfo) GetMounterInfo(mounterId string, aesSecret []byte) (*mounts.MounterNodeInfo, error) {
	mounterInfoKey := "mounter." + mounterId
	mountInfoRaw := ni.Data[mounterInfoKey]
	if mountInfoRaw == "" {
		return nil, fmt.Errorf("node_info: key '%s' missing", mounterInfoKey)
	}
	mountInfo, err := mounts.DeserializeMounterNodeInfo(mountInfoRaw, aesSecret)
	if err != nil {
		return nil, err
	}
	return &mountInfo, nil
}

package voldbwrapper

import (
	"context"
	"encoding/json"
	"fmt"
)

const mainVolumeModule = "volume.json"

type VolumeInfo struct {
	MounterId     string            `json:"mount_id"`
	TransportId   string            `json:"transport_id"`
	VolumeContext map[string]string `json:"volume_context"`
	AddressIds    []string          `json:"address_ids"`
	StorageType   string            `json:"storage_type"`
	StorageData   string            `json:"volume"`
	ShareType     string            `json:"share_type"`
	ShareData     string            `json:"share_data"`
}

func (w Wrapper) SetVolumeInfo(ctx context.Context, info VolumeInfo) error {
	encoded, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return w.entry.SetModule(ctx, mainVolumeModule, encoded)
}

func (w Wrapper) GetVolumeInfo(ctx context.Context) (*VolumeInfo, error) {
	content, err := w.entry.GetModule(ctx, mainVolumeModule)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, nil
	}
	var result VolumeInfo
	err = json.Unmarshal(content, &result)
	if err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return &result, nil
}

package voldbwrapper

import (
	"context"
	"encoding/json"
	"fmt"
)

const mainVolumeModule = "volume.json"

type VolumeInfo struct {
	VolumeType string
	Data json.RawMessage
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

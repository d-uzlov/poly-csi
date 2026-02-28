package cntriscsiadm

import (
	"encoding/json"
	"fmt"
)

type VolumeData struct {
	Iqn string `json:"iqn"`
}

func (d *VolumeData) Validate() error {
	if d.Iqn == "" {
		return fmt.Errorf("missing iqn")
	}
	return nil
}

func parseVolumeData(serialized string) (VolumeData, error) {
	var result VolumeData
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return VolumeData{}, err
	}
	err = result.Validate()
	if err != nil {
		return VolumeData{}, err
	}
	return result, nil
}

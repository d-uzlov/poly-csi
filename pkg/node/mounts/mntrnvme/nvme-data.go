package mntrnvme

import (
	"encoding/json"
	"fmt"
)

type VolumeData struct {
	Nqn string `json:"nqn"`
}

func (d *VolumeData) Validate() error {
	if d.Nqn == "" {
		return fmt.Errorf("missing nqn")
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

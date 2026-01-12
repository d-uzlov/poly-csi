package zfsdata

import (
	"encoding/json"
	"fmt"
)

type ZfsData struct {
	Path string `json:"path"`
}

func (d ZfsData) Serialize() (string, error) {
	encoded, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func Parse(encoded string) (ZfsData, error) {
	var result ZfsData
	err := json.Unmarshal([]byte(encoded), &result)
	if err != nil {
		return ZfsData{}, fmt.Errorf("unmarshal json: %w", err)
	}
	if result.Path == "" {
		return ZfsData{}, fmt.Errorf("zfs dataset path is empty")
	}
	return result, nil
}

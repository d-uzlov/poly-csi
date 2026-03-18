package mplocal

import (
	"encoding/json"
	"fmt"
)

type Data struct {
	MapName string `json:"map_name"`
}

func (d *Data) Validate() error {
	if d.MapName == "" {
		return fmt.Errorf("missing map_name")
	}
	return nil
}

func parseData(serialized string) (Data, error) {
	var result Data
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return Data{}, err
	}
	err = result.Validate()
	if err != nil {
		return Data{}, err
	}
	return result, nil
}

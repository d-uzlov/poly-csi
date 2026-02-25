package mplocal

import (
	"encoding/json"
	"fmt"
)

type Data struct {
	Wwid string `json:"wwid"`
}

func (d *Data) Validate() error {
	if d.Wwid == "" {
		return fmt.Errorf("missing wwid")
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

package shrtargetcli

import (
	"encoding/json"
	"fmt"
)

type shareContext struct {
	Iqn           string `json:"iqn"`
	Port          string `json:"port"`
	Transport     string `json:"transport"`
	BackstoreName string `json:"backstore_name"`
	AuthType      string `json:"auth_type"`
}

func (c shareContext) serialize() string {
	encoded, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func (c shareContext) validate() error {
	if c.Iqn == "" {
		return fmt.Errorf("missing iqn")
	}
	if c.BackstoreName == "" {
		return fmt.Errorf("missing backstoreName")
	}
	if c.Port == "" {
		return fmt.Errorf("missing port")
	}
	if c.Transport == "" {
		return fmt.Errorf("missing transport")
	}
	return nil
}

func parseShareData(data string) (shareContext, error) {
	var result shareContext
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		return shareContext{}, err
	}
	err = result.validate()
	if err != nil {
		return shareContext{}, err
	}
	return result, nil
}

package shrnvmetkeeper

import (
	"encoding/json"
	"fmt"
)

type shareContext struct {
	Nqn       string `json:"nqn"`
	Port      string `json:"port"`
	Transport string `json:"transport"`
	IoQueues  string `json:"io_queues"`
	AuthType  string `json:"auth_type"`
}

func (c shareContext) serialize() string {
	encoded, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func (c shareContext) validate() error {
	if c.Nqn == "" {
		return fmt.Errorf("missing nqn")
	}
	if c.Port == "" {
		return fmt.Errorf("missing port")
	}
	if c.Transport == "" {
		return fmt.Errorf("missing transport")
	}
	if c.AuthType == "" {
		return fmt.Errorf("missing auth_type")
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

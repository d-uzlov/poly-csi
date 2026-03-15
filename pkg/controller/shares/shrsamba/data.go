package shrsamba

import (
	"encoding/json"
	"fmt"
)

type shareContext struct {
	ConfigFile string `json:"config_file"`
	ShareName  string `json:"share_name"`
	UserName   string `json:"user_name"`
	UserPass   string `json:"user_pass"`
}

func (c shareContext) serialize() string {
	encoded, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func (c shareContext) validate() error {
	if c.ConfigFile == "" {
		return fmt.Errorf("missing config_file")
	}
	if c.ShareName == "" {
		return fmt.Errorf("missing share_name")
	}
	if c.UserName == "" {
		return fmt.Errorf("missing user_name")
	}
	if c.UserPass == "" {
		return fmt.Errorf("missing user_pass")
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

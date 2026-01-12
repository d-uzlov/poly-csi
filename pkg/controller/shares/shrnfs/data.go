package shrnfs

import (
	"encoding/json"
	"fmt"
)

type shareContext struct {
	ExportsFile string `json:"exports_file"`
	Folder      string `json:"folder"`
}

func (c shareContext) serialize() string {
	encoded, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func (c shareContext) validate() error {
	if c.ExportsFile == "" {
		return fmt.Errorf("missing exports_file")
	}
	if c.Folder == "" {
		return fmt.Errorf("missing folder")
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

package shrzfssharenfs

import (
	"encoding/json"
	"fmt"
)

type shareContext struct {
	Dataset string `json:"dataset"`
	Folder  string `json:"folder"`
}

func (c shareContext) serialize() string {
	encoded, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func (c shareContext) validate() error {
	if c.Dataset == "" {
		return fmt.Errorf("missing dataset")
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

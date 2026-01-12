package shrtniscsi

import (
	"encoding/json"
	"fmt"
)

type shareContext struct {
	Iqn  string
	Zvol string
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
	if c.Zvol == "" {
		return fmt.Errorf("missing zvol")
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

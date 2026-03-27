package strlvm

import (
	"fmt"
	"strings"
)

type params struct {
	vgName string
}

const paramsPrefix = "lvm."

const (
	keyVgName = "lvm.vgName"
)

func parseParams(static map[string]string, mutable map[string]string) (*params, error) {
	result := params{}
	for k, v := range static {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyVgName:
			result.vgName = v
		default:
			return nil, fmt.Errorf("unknown key '%s'", k)
		}
	}
	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyVgName:
			return nil, fmt.Errorf("key %s is not allowed in mutable params", k)
		default:
			return nil, fmt.Errorf("unknown key '%s'", k)
		}
	}

	// no defaults needed

	err := result.validate()
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (p params) validate() error {
	if p.vgName == "" {
		return fmt.Errorf("%s is empty", keyVgName)
	}
	return nil
}

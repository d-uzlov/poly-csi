package strfolder

import (
	"fmt"
	"strings"
)

type params struct {
	parentFolder string
}

const paramsPrefix = "folder."

const (
	keyParentFolder = "folder.parentFolder"
)

func parseParams(static map[string]string, mutable map[string]string) (params, error) {
	result := params{}

	for k, v := range static {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyParentFolder:
			result.parentFolder = v
		default:
			return params{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyParentFolder:
			return params{}, fmt.Errorf("key %s is not allowed in mutable params", k)
		default:
			return params{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	err := result.validate()
	if err != nil {
		return params{}, err
	}

	return result, nil
}

func (p params) validate() error {
	if p.parentFolder == "" {
		return fmt.Errorf("parentDataset is empty")
	}
	return nil
}

package strzfsdataset

import (
	"fmt"
	"strings"
)

type commands struct {
	zfsCmd     []string
}

func parseCommands(src map[string][]string) (commands, error) {
	result := commands{}

	result.zfsCmd = src["zfs"]
	if len(result.zfsCmd) == 0 {
		result.zfsCmd = []string{"sudo", "zfs"}
	}

	return result, nil
}

type params struct {
	parentDataset string
	customOptions string
}

const paramsPrefix = "zfs."

const (
	keyParentDataset = "zfs.parentDataset"
	keyCustomOptions = "zfs.customOptions"
)

func parseParams(static map[string]string, mutable map[string]string) (*params, error) {
	result := params{}
	for k, v := range static {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyParentDataset:
			result.parentDataset = v
		case keyCustomOptions:
			result.customOptions = v
		default:
			return nil, fmt.Errorf("unknown key '%s'", k)
		}
	}
	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyParentDataset:
			return nil, fmt.Errorf("key %s is not allowed in mutable params", k)
		case keyCustomOptions:
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
	if p.parentDataset == "" {
		return fmt.Errorf("parentDataset is empty")
	}
	return nil
}

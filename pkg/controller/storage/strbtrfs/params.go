package strbtrfs

import (
	"fmt"
	"strings"
)

type commands struct {
	btrfsCmd []string
}

func parseCommands(src map[string][]string) (commands, error) {
	result := commands{}

	result.btrfsCmd = src["btrfs"]
	if len(result.btrfsCmd) == 0 {
		result.btrfsCmd = []string{"sudo", "btrfs"}
	}

	return result, nil
}

type params struct {
	parentPath    string
	customOptions string
}

const paramsPrefix = "btrfs."

const (
	keyParentPath    = "btrfs.parentPath"
	keyCustomOptions = "btrfs.customOptions"
)

func parseParams(static map[string]string, mutable map[string]string) (*params, error) {
	result := params{}
	for k, v := range static {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyParentPath:
			result.parentPath = v
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
		case keyParentPath:
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
	if p.parentPath == "" {
		return fmt.Errorf("%s is empty", keyParentPath)
	}
	return nil
}

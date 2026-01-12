package shrzfssharenfs

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
	shareOptions string
}

const paramsPrefix = "nfs."

const (
	keyShareOptions = "nfs.shareOptions"
)

func parseParams(static map[string]string, mutable map[string]string) (params, error) {
	result := params{}

	for k, v := range static {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyShareOptions:
			result.shareOptions = v
		default:
			return params{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyShareOptions:
			return params{}, fmt.Errorf("key %s is not allowed in mutable params", k)
		default:
			return params{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	// fill defaults
	if result.shareOptions == "" {
		result.shareOptions = "rw,no_subtree_check,no_root_squash"
	}

	return result, nil
}

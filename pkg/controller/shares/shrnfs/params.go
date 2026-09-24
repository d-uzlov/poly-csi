package shrnfs

import (
	"fmt"
	"strings"
)

var nfsReloadCmd = []string{"nfs_reload"}

type globalParams struct {
}

type Params struct {
	shareOptions string
}

const paramsPrefix = "nfs."

const (
	keyShareOptions = "nfs.shareOptions"
)

func parseGlobalParams(src map[string]string) (globalParams, error) {
	result := globalParams{}

	for k := range src {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		default:
			return globalParams{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	return result, nil
}

func parseParams(static map[string]string, mutable map[string]string) (Params, error) {
	result := Params{}

	for k, v := range static {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyShareOptions:
			result.shareOptions = v
		default:
			return Params{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyShareOptions:
			return Params{}, fmt.Errorf("key %s is not allowed in mutable params", k)
		default:
			return Params{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	// fill defaults
	if result.shareOptions == "" {
		result.shareOptions = "rw,no_subtree_check,no_root_squash"
	}

	// no need to validate anything

	return result, nil
}

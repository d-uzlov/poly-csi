package shrnbd

import (
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

const (
	configDirKey = "nbd.config_dir"
)

type globalParams struct {
	configDir string
}

type params struct {
	ConnectPort string `map:"connectPort"`
}

const paramsPrefix = "nbd."

func parseGlobalParams(src map[string]string) (globalParams, error) {
	result := globalParams{}

	for k, v := range src {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case configDirKey:
			result.configDir = v
		default:
			return globalParams{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	if result.configDir == "" {
		return result, fmt.Errorf("missing configDir")
	}

	return result, nil
}

func parseParams(static map[string]string, mutable map[string]string) (params, error) {
	result := params{}
	err := serialmaps.ParseRemove(static, paramsPrefix, true, &result)
	if err != nil {
		return params{}, fmt.Errorf("static params: %w", err)
	}

	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		default:
			return params{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	// no need to validate anything

	return result, nil
}

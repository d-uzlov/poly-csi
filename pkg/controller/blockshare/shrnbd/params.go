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
	ConnectPort int `map:"connectPort"`
}

const paramsPrefix = "nbd"

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

func parseParams(parser *serialmaps.Parser) (params, error) {
	var result params
	err := parser.Parse(&result)
	if err != nil {
		return params{}, fmt.Errorf("parse params: %w", err)
	}

	return result, nil
}

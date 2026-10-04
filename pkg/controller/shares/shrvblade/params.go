package shrvblade

import (
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

const (
	configDirKey  = "vblade.config_dir"
	minShelfKey   = "vblade.minShelf"
	maxShelfKey   = "vblade.maxShelf"
	interfacesKey = "vblade.interfaces"
)

type globalParams struct {
	ConfigDir  string
	MinShelf   string
	MaxShelf   string
	Interfaces string
}

type Params struct {
	Interfaces string `map:"interfaces"`
}

const paramsPrefix = "vblade."

func parseGlobalParams(src map[string]string) (globalParams, error) {
	result := globalParams{}

	for k, v := range src {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case configDirKey:
			result.ConfigDir = v
		case minShelfKey:
			result.MinShelf = v
		case maxShelfKey:
			result.MaxShelf = v
		case interfacesKey:
			result.Interfaces = v
		default:
			return globalParams{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	if result.ConfigDir == "" {
		return result, fmt.Errorf("missing configDir")
	}
	if result.MinShelf == "" {
		return result, fmt.Errorf("missing MinShelf")
	}
	if result.MaxShelf == "" {
		return result, fmt.Errorf("missing MaxShelf")
	}
	if result.Interfaces == "" {
		return result, fmt.Errorf("missing Interfaces")
	}

	return result, nil
}

func parseParams(static map[string]string, mutable map[string]string) (Params, error) {
	result := Params{}
	err := serialmaps.ParseRemove(static, paramsPrefix, true, &result)
	if err != nil {
		return Params{}, fmt.Errorf("static params: %w", err)
	}

	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		default:
			return Params{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	// no need to validate anything

	return result, nil
}

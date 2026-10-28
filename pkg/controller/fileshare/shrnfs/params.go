package shrnfs

import (
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

var nfsReloadCmd = []string{"nfs_reload"}

type globalParams struct {
}

type Params struct {
	ShareOptions string `map:"shareOptions"`
}

const paramsPrefix = "nfs"

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

func parseParams(parser *serialmaps.Parser) (Params, error) {
	var result Params
	err := parser.Parse(&result)
	if err != nil {
		return Params{}, fmt.Errorf("parse params: %w", err)
	}

	if result.ShareOptions == "" {
		result.ShareOptions = "rw,no_subtree_check,no_root_squash"
	}

	return result, nil
}

package shrsamba

import (
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

var createUserCmd = []string{"user_create"}
var deleteUserCmd = []string{"user_delete"}
var sambaSetPassCmd = []string{"samba_set_password"}
var sambaDeleteUserCmd = []string{"samba_user_delete"}
var sambaRefreshCmd = []string{"samba_refresh"}

const (
	sambaConfigDirKey = "samba_config_dir"
)

type globalParams struct {
	sambaConfigDir string
}

type params struct {
}

const paramsPrefix = "samba."

func parseGlobalParams(src map[string]string) (globalParams, error) {
	result := globalParams{}

	for k, v := range src {
		switch k {
		case sambaConfigDirKey:
			result.sambaConfigDir = v
		}
	}

	if result.sambaConfigDir == "" {
		return globalParams{}, fmt.Errorf("missing key %s", sambaConfigDirKey)
	}

	return result, nil
}

func parseParams(static map[string]string, mutable map[string]string) (*params, error) {
	result := params{}
	err := serialmaps.ParseRemove(static, paramsPrefix, true, &result)
	if err != nil {
		return nil, fmt.Errorf("static params: %w", err)
	}

	resMutable := params{}
	err = serialmaps.ParseRemove(mutable, paramsPrefix, true, &resMutable)
	if err != nil {
		return nil, fmt.Errorf("mutable params: %w", err)
	}

	for k := range static {
		if strings.HasPrefix(k, paramsPrefix) {
			return nil, fmt.Errorf("unknown key: %s", k)
		}
	}

	for k := range static {
		if strings.HasPrefix(k, paramsPrefix) {
			return nil, fmt.Errorf("unknown mutable key: %s", k)
		}
	}

	return &result, nil
}

package shrsamba

import (
	"fmt"

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

func parseParams(parser *serialmaps.Parser) (*params, error) {
	var result params
	err := parser.Parse(&result)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	return &result, nil
}

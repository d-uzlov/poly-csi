package shrsamba

import (
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

const (
	createUserCmdKey       = "user_create"
	deleteUserCmdKey       = "user_delete"
	sambaSetPasswordCmdKey = "samba_set_password"
	sambaDeleteUserCmdKey  = "samba_user_delete"
	sambaRefreshCmdKey     = "samba_refresh"

	sambaConfigDirKey = "samba_config_dir"
)

type commands struct {
	createUser      []string
	deleteUser      []string
	sambaSetPass    []string
	sambaDeleteUser []string
	smbRefresh      []string
}

func parseCommands(src map[string][]string) (commands, error) {
	createUserCmd := src[createUserCmdKey]
	if len(createUserCmdKey) == 0 {
		return commands{}, fmt.Errorf("missing command %s", createUserCmdKey)
	}
	deleteUserCmd := src[deleteUserCmdKey]
	if len(deleteUserCmd) == 0 {
		return commands{}, fmt.Errorf("missing command %s", deleteUserCmdKey)
	}
	sambaSetPasswordCmd := src[sambaSetPasswordCmdKey]
	if len(sambaSetPasswordCmd) == 0 {
		return commands{}, fmt.Errorf("missing command %s", sambaSetPasswordCmdKey)
	}
	sambaDeleteUserCmd := src[sambaDeleteUserCmdKey]
	if len(sambaDeleteUserCmd) == 0 {
		return commands{}, fmt.Errorf("missing command %s", sambaDeleteUserCmdKey)
	}
	sambaRefreshCmd := src[sambaRefreshCmdKey]
	if len(sambaRefreshCmd) == 0 {
		return commands{}, fmt.Errorf("missing command %s", sambaRefreshCmdKey)
	}

	return commands{
		createUser:      createUserCmd,
		deleteUser:      deleteUserCmd,
		sambaSetPass:    sambaSetPasswordCmd,
		sambaDeleteUser: sambaDeleteUserCmd,
		smbRefresh:      sambaRefreshCmd,
	}, nil
}

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

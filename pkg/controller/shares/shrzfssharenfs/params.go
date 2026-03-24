package shrzfssharenfs

import (
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type commands struct {
	zfsCmd []string
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
	ShareOptions string `map:"shareOptions"`
}

const paramsPrefix = "nfs."

func parseParams(static map[string]string, mutable map[string]string) (params, error) {
	result := params{}
	err := serialmaps.ParseRemove(static, paramsPrefix, true, &result)
	if err != nil {
		return params{}, fmt.Errorf("static params: %w", err)
	}

	resMutable := params{}
	err = serialmaps.ParseRemove(mutable, paramsPrefix, true, &resMutable)
	if err != nil {
		return params{}, fmt.Errorf("mutable params: %w", err)
	}

	if resMutable.ShareOptions != "" {
		result.ShareOptions = resMutable.ShareOptions
	}

	for k := range static {
		if strings.HasPrefix(k, paramsPrefix) {
			return params{}, fmt.Errorf("unknown key: %s", k)
		}
	}

	for k := range static {
		if strings.HasPrefix(k, paramsPrefix) {
			return params{}, fmt.Errorf("unknown mutable key: %s", k)
		}
	}

	return result, nil
}

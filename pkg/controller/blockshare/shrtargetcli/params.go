package shrtargetcli

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

var readyCmd = []string{"targetcli_ready"}

func testReady(ctx context.Context, tr transport.Transport) bool {
	runResult, err := tr.RunCommand(ctx, readyCmd)
	if err != nil {
		return false
	}
	if runResult.ExitCode != 0 {
		return false
	}
	return true
}

type globalParams struct {
	iqnBase string
}

type params struct {
	Transport  string `map:"transport"`
	ListenPort int    `map:"listenPort"`
	AuthType   string `map:"authType"`
}

const paramsPrefix = "lio"

const (
	keyIqnBase = "iqnBase"
)

const (
	authTypeNone          = "none"
	authTypeIqn           = "iqn"
	authTypeChapProvision = "chap-provision"
)

var allowedAuth = []string{authTypeNone, authTypeIqn, authTypeChapProvision}

func parseGlobalParams(src map[string]string) (globalParams, error) {
	result := globalParams{}

	for k, v := range src {
		switch k {
		case keyIqnBase:
			result.iqnBase = v
		default:
			if !strings.HasPrefix(k, paramsPrefix) {
				continue
			}
			return globalParams{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	// validate
	if result.iqnBase == "" {
		return globalParams{}, fmt.Errorf("iqnBase is empty")
	}

	return result, nil
}

func parseParams(static map[string]string, mutable map[string]string) (*params, error) {
	result := params{}
	err := serialmaps.Parse(static, paramsPrefix, &result, serialmaps.WithRemove(true))
	if err != nil {
		return nil, fmt.Errorf("static params: %w", err)
	}

	resMutable := params{}
	err = serialmaps.Parse(mutable, paramsPrefix, &resMutable, serialmaps.WithRemove(true))
	if err != nil {
		return nil, fmt.Errorf("mutable params: %w", err)
	}

	if resMutable.Transport != "" {
		return nil, fmt.Errorf("transport is not mutable")
	}
	if resMutable.AuthType != "" {
		return nil, fmt.Errorf("auth type is not mutable")
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

func (p params) validate() error {
	if p.Transport == "" {
		return fmt.Errorf("transport is empty")
	}
	if p.AuthType == "" {
		return fmt.Errorf("authType is empty")
	}
	switch p.AuthType {
	case "":
		return fmt.Errorf("authType is empty")
	case authTypeNone, authTypeIqn, authTypeChapProvision:
		// ok
	default:
		return fmt.Errorf("auth: unknown type: '%s', must be one of: %v", p.AuthType, allowedAuth)
	}
	return nil
}

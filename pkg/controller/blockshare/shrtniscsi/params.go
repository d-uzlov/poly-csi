package shrtniscsi

import (
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type globalParams struct {
	iqnBase string
}

type params struct {
	Transport  string `map:"transport"`
	ListenPort string `map:"listenPort"`
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

	// no defaults needed

	// validate
	if result.iqnBase == "" {
		return globalParams{}, fmt.Errorf("iqnBase is empty")
	}

	return result, nil
}

func parseParams(parser *serialmaps.Parser) (*params, error) {
	var result params
	err := parser.Parse(&result)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	err = result.validate()
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (p params) validate() error {
	if p.Transport == "" {
		return fmt.Errorf("transport is empty")
	}
	if p.ListenPort == "" {
		return fmt.Errorf("listenPort is empty")
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

package shrtniscsi

import (
	"fmt"
	"strings"
)

type globalParams struct {
	iqnBase          string
}

type params struct {
	transport        string
	listenPort       string
	authType         string
}

const paramsPrefix = "lio."

const (
	keyIqnBase          = "iqnBase"

	keyTransport  = "lio.transport"
	keyListenPort = "lio.listenPort"
	keyAuthType   = "lio.authType"
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

func parseParams(static map[string]string, mutable map[string]string) (*params, error) {
	result := params{}

	for k, v := range static {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyTransport:
			result.transport = v
		case keyListenPort:
			result.listenPort = v
		case keyAuthType:
			result.authType = v
		default:
			return nil, fmt.Errorf("unknown key '%s'", k)
		}
	}

	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyTransport:
			return nil, fmt.Errorf("key %s is not allowed in mutable params", k)
		case keyListenPort:
			return nil, fmt.Errorf("key %s is not allowed in mutable params", k)
		case keyAuthType:
			return nil, fmt.Errorf("key %s is not allowed in mutable params", k)
		default:
			return nil, fmt.Errorf("unknown key '%s'", k)
		}
	}

	// fill defaults
	if result.transport == "" {
		result.transport = "tcp"
	}
	if result.listenPort == "" {
		result.listenPort = "3260"
	}
	if result.authType == "" {
		result.authType = authTypeChapProvision
	}

	err := result.validate()
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (p params) validate() error {
	if p.transport == "" {
		return fmt.Errorf("transport is empty")
	}
	if p.listenPort == "" {
		return fmt.Errorf("listenPort is empty")
	}
	if p.authType == "" {
		return fmt.Errorf("authType is empty")
	}
	switch p.authType {
	case "":
		return fmt.Errorf("authType is empty")
	case authTypeNone, authTypeIqn, authTypeChapProvision:
		// ok
	default:
		return fmt.Errorf("auth: unknown type: '%s', must be one of: %v", p.authType, allowedAuth)
	}
	return nil
}

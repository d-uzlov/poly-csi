package shrnvmetkeeper

import (
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

var nvmetKeeperCmd = []string{"nvmet_keeper"}

type globalParams struct {
	nqnBase string
}

type params struct {
	TransportType string `map:"transportType"`
	NvmetPortName string `map:"nvmetPortName"`
	ConnectPort   int    `map:"connectPort"`
	AuthType      string `map:"authType"`

	// TODO add fallback to reading ListenPort from current nvmet state
}

const paramsPrefix = "nvmet"

const (
	keyNqnBase = "nqnBase"
)

const (
	authTypeNone          = "none"
	authTypeNqn           = "nqn"
	authTypeChapProvision = "chap-provision"
)

var allowedAuth = []string{authTypeNone, authTypeNqn, authTypeChapProvision}

func parseGlobalParams(src map[string]string) (globalParams, error) {
	result := globalParams{}

	for k, v := range src {
		switch k {
		case keyNqnBase:
			result.nqnBase = v
		default:
			if !strings.HasPrefix(k, paramsPrefix) {
				continue
			}
			return globalParams{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	// fill defaults
	if result.nqnBase == "" {
		return globalParams{}, fmt.Errorf("missing nqnBase")
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

func (p params) validate() error {
	if p.TransportType == "" {
		return fmt.Errorf("missing key transport type")
	}
	if p.AuthType == "" {
		return fmt.Errorf("missing auth type")
	}
	switch p.AuthType {
	case "":
		return fmt.Errorf("authType is empty")
	case authTypeNone, authTypeNqn, authTypeChapProvision:
		// ok
	default:
		return fmt.Errorf("auth: unknown type: '%s', must be one of: %v", p.AuthType, allowedAuth)
	}
	return nil
}

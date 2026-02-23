package shrnvmetkeeper

import (
	"fmt"
	"strings"
)

type commandsConfig struct {
	nvmetKeeperCmd []string
}

func parseCommands(commands map[string][]string) (commandsConfig, error) {
	nvmetKeeperCmd := commands["nvmet_keeper"]
	if len(nvmetKeeperCmd) == 0 {
		return commandsConfig{}, fmt.Errorf("missing command 'nvmet_keeper'")
	}

	return commandsConfig{
		nvmetKeeperCmd: nvmetKeeperCmd,
	}, nil
}

type globalParams struct {
	nqnBase string
}

type params struct {
	transportType string
	nvmetPort     string
	// TODO add fallback to reading listenPort from current nvmet state
	listenPort string
	ioQueues   string
	authType   string
}

const paramsPrefix = "nvmet."

const (
	keyNqnBase = "nqnBase"

	keyTransport   = "nvmet.transportType"
	keyNvmetPort   = "nvmet.nvmetPortName"
	keyConnectPort = "nvmet.connectPort"
	keyIoQueues    = "nvmet.ioQueues"
	keyAuthType    = "nvmet.authType"
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

func parseParams(static map[string]string, mutable map[string]string) (params, error) {
	result := params{}

	for k, v := range static {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyTransport:
			result.transportType = v
		case keyNvmetPort:
			result.nvmetPort = v
		case keyConnectPort:
			result.listenPort = v
		case keyIoQueues:
			result.ioQueues = v
		case keyAuthType:
			result.authType = v
		default:
			return params{}, fmt.Errorf("static params: unknown key '%s'", k)
		}
	}

	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case keyTransport:
			return params{}, fmt.Errorf("key %s is not allowed in mutable params", k)
		case keyNvmetPort:
			return params{}, fmt.Errorf("key %s is not allowed in mutable params", k)
		case keyConnectPort:
			return params{}, fmt.Errorf("key %s is not allowed in mutable params", k)
		case keyIoQueues:
			return params{}, fmt.Errorf("key %s is not allowed in mutable params", k)
		case keyAuthType:
			return params{}, fmt.Errorf("key %s is not allowed in mutable params", k)
		default:
			return params{}, fmt.Errorf("mutable params: unknown key '%s'", k)
		}
	}

	if result.transportType == "" {
		result.transportType = "tcp"
	}

	err := result.validate()
	if err != nil {
		return params{}, err
	}

	return result, nil
}

func (p params) validate() error {
	if p.transportType == "" {
		return fmt.Errorf("missing key %s", keyTransport)
	}
	if p.listenPort == "" {
		return fmt.Errorf("missing key %s", keyConnectPort)
	}
	if p.authType == "" {
		return fmt.Errorf("missing key %s", keyAuthType)
	}
	switch p.authType {
	case "":
		return fmt.Errorf("authType is empty")
	case authTypeNone, authTypeNqn, authTypeChapProvision:
		// ok
	default:
		return fmt.Errorf("%s: unknown type: '%s', must be one of: %v", keyAuthType, p.authType, allowedAuth)
	}
	return nil
}

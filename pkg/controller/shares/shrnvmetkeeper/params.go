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
	nqnBase    string
	transport  string
	nvmetPort  string
	listenPort string
	ioQueues   string
}

type params struct {
	transport  string
	nvmetPort  string
	listenPort string
	ioQueues   string
}

const paramsPrefix = "nvmet."

const (
	keyNqnBase = "nqnBase"

	keyTransport  = "nvmet.transport"
	keyNvmetPort  = "nvmet.nvmetPort"
	keyListenPort = "nvmet.listenPort"
	keyIoQueues   = "nvmet.ioQueues"
)

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
			result.transport = v
		case keyNvmetPort:
			result.nvmetPort = v
		case keyListenPort:
			result.listenPort = v
		case keyIoQueues:
			result.ioQueues = v
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
		case keyListenPort:
			return params{}, fmt.Errorf("key %s is not allowed in mutable params", k)
		case keyIoQueues:
			return params{}, fmt.Errorf("key %s is not allowed in mutable params", k)
		default:
			return params{}, fmt.Errorf("mutable params: unknown key '%s'", k)
		}
	}

	// fill defaults
	if result.transport == "" {
		result.transport = "tcp"
	}
	if result.nvmetPort == "" {
		result.nvmetPort = "1"
	}
	if result.listenPort == "" {
		result.listenPort = "4420"
	}
	if result.nvmetPort == "" {
		result.nvmetPort = "1"
	}

	// no need for validations

	return result, nil
}

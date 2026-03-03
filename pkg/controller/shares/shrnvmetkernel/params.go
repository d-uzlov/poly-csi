package shrnvmetkernel

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

type commands struct {
	readyCmd []string
	saveCmd  []string
}

func parseCommands(src map[string][]string) (commands, error) {
	saveCmd := src["nvmet_save"]
	if len(saveCmd) == 0 {
		return commands{}, fmt.Errorf("missing command 'nvmet_save'")
	}
	readyCmd := src["nvmet_ready"]

	return commands{
		saveCmd:  saveCmd,
		readyCmd: readyCmd,
	}, nil
}

func (c commands) ready(ctx context.Context, tr transport.Transport) bool {
	if len(c.readyCmd) == 0 {
		return true
	}

	runResult, err := tr.RunCommand(ctx, c.readyCmd)
	if err != nil {
		return false
	}
	if runResult.ExitCode != 0 {
		return false
	}
	return true
}

func (c commands) save(ctx context.Context, tr transport.Transport) error {
	if len(c.saveCmd) == 0 {
		return fmt.Errorf("save command is not defined")
	}

	runResult, err := tr.RunCommand(ctx, c.saveCmd)
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
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

	// validate
	if result.nqnBase == "" {
		return globalParams{}, fmt.Errorf("nqnBase is missing")
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

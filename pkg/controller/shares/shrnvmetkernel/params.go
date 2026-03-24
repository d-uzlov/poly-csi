package shrnvmetkernel

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
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
	TransportType string `map:"transportType"`
	NvmetPortName string `map:"nvmetPortName"`
	ConnectPort   string `map:"connectPort"`
	IoQueues      string `map:"ioQueues"`
}

const paramsPrefix = "nvmet."

const (
	keyNqnBase = "nqnBase"
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

	if resMutable.TransportType != "" {
		return nil, fmt.Errorf("transport type is not mutable")
	}
	if resMutable.NvmetPortName != "" {
		return nil, fmt.Errorf("port name is not mutable")
	}
	if resMutable.ConnectPort != "" {
		return nil, fmt.Errorf("connect port is not mutable")
	}
	if resMutable.IoQueues != "" {
		return nil, fmt.Errorf("io queues is not mutable")
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

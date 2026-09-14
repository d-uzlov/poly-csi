package shrnvmetkernel

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

var readyCmd = []string{"nvmet_ready"}
var saveCmd = []string{"nvmet_save"}

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

func saveState(ctx context.Context, tr transport.Transport) error {
	runResult, err := tr.RunCommand(ctx, saveCmd)
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
}

type params struct {
	TransportType string `map:"transportType"`
	NvmetPortName string `map:"nvmetPortName"`
	ConnectPort   string `map:"connectPort"`
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

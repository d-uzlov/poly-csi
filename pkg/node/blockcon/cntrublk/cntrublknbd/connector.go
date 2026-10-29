package cntrublknbd

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/node/blockcon/cntrublk"
	"meoe.io/poly-csi/pkg/utils/ublk"
)

type connector struct {
	blockapi.NoInfo

	conf    blockapi.CommonConfig
	sv      cntrublk.Supervisor
	presets map[string]ublk.Preset
}

func New(sv cntrublk.Supervisor, conf blockapi.CommonConfig) (blockapi.Manager, error) {
	presets := map[string]ublk.Preset{}
	for k, v := range conf.Presets {
		p, err := cntrublk.ParsePreset(v)
		if err != nil {
			return nil, fmt.Errorf("preset '%s': %w", k, err)
		}
		presets[k] = p
	}
	return &connector{
		sv:      sv,
		conf:    conf,
		presets: presets,
	}, nil
}

func (c *connector) Capabilities(ctx context.Context) (blockapi.Capabilities, error) {
	return blockapi.Capabilities{
		Protocol:    blockspec.Nbd,
		HasNodeInfo: false,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req blockapi.ConnectRequest) (blockapi.Handle, error) {
	proto, ok := req.Proto.(blockspec.NbdShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch: got %T", req.Proto)
	}

	var preset ublk.Preset
	if req.PresetName == "" {
		preset = ublk.Preset{
			Queues:     1,
			QueueDepth: 127,
			BufSize:    512 * 1024,
		}
	} else {
		preset, ok = c.presets[req.PresetName]
		if !ok {
			return nil, fmt.Errorf("preset '%s': not found (config has %v)", req.PresetName, maps.Keys(c.presets))
		}
	}

	return &state{
		tr:        c.conf.Tr,
		sv:        c.sv,
		devFolder: c.conf.DevFolder,
		Uuid:      req.Uuid,
		Address:   proto.ServerAddress,
		Pc:        proto,
		Preset:    preset,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData json.RawMessage) (blockapi.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.conf.Tr
	res.sv = c.sv
	res.devFolder = c.conf.DevFolder

	return res, nil
}

package cntrnvme

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/blockspec"
)

type connector struct {
	blockapi.NoInfo

	commonConfig blockapi.CommonConfig
	snip         NvmeSnippets
	config       Config
	presets      map[string]PresetData
}

type PresetData struct {
	IoQueues              string `map:"ioQueues"`
	KeepAliVeTimeout      string `map:"ioQueues"`
	ControllerLossTimeout string `map:"controllerLossTimeoutSec"`
	ReconnectDelay        string `map:"reconnectDelaySec"`
}

func New(conf blockapi.CommonConfig) (blockapi.Manager, error) {
	result := &connector{
		presets: map[string]PresetData{},
	}
	result.commonConfig = conf

	err := conf.Cfg.Decode(&result.config)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := result.config.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	for k, v := range conf.Presets {
		var preset PresetData
		err = v.Decode(&preset)
		if err != nil {
			return nil, fmt.Errorf("parse preset '%s': %w", k, err)
		}
		result.presets[k] = preset
	}

	result.snip = newNvmeSnippets(conf.Tr, result.config.NvmeCmd, conf.DevFolder, result.config.ParseKernelLog)

	return result, nil
}

func (c *connector) Capabilities(ctx context.Context) (blockapi.Capabilities, error) {
	return blockapi.Capabilities{
		Protocol:    blockspec.Nvme,
		HasNodeInfo: false,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req blockapi.ConnectRequest) (blockapi.Handle, error) {
	proto, ok := req.Proto.(blockspec.NvmeShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch: got %T", req.Proto)
	}

	preset, ok := c.presets[req.PresetName]
	if !ok {
		return nil, fmt.Errorf("preset not found: '%s'", req.PresetName)
	}

	cp := ConnectParams{
		TargetNqn:     proto.TargetNqn,
		HostNqn:       proto.HostNqn,
		HostId:        proto.HostId,
		Port:          proto.ConnectPort,
		NvmeTransport: proto.TransportType,
	}
	var auth *ConnectAuth
	if proto.HostKey != "" && proto.ControllerKey != "" {
		auth = &ConnectAuth{
			HostKey:       proto.HostKey,
			ControllerKey: proto.ControllerKey,
		}
	}
	extra := ConnectExtra{
		IoQueues:              preset.IoQueues,
		KeepAliveTimeout:      preset.KeepAliVeTimeout,
		ControllerLossTimeout: preset.ControllerLossTimeout,
		ReconnectDelay:        preset.ReconnectDelay,
	}

	ecs := []ExpectedConnection{}
	for _, ao := range proto.ServerAddress {
		ecs = append(ecs, ExpectedConnection{
			Addr: ao,
		})
	}

	return &state{
		tr:          c.commonConfig.Tr,
		snip:        &c.snip,
		devFolder:   c.commonConfig.DevFolder,
		Params:      cp,
		Auth:        auth,
		ConExtra:    extra,
		Connections: ecs,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData json.RawMessage) (blockapi.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.commonConfig.Tr
	res.devFolder = c.snip.devFolder
	res.snip = &c.snip

	return res, nil
}

func getIpFromNvmeAddr(full string) string {
	for v := range strings.SplitSeq(full, ",") {
		if suffix, ok := strings.CutPrefix(v, "traddr="); ok {
			return suffix
		}
	}
	return ""
}

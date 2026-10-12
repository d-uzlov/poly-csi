package cntrnvme

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/node/blockcon"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type connector struct {
	blockcon.NoInfo

	commonConfig blockcon.CommonConfig
	snip         NvmeSnippets
	config       Config
}

const mapPrefix = "nvme"

type PresetData struct {
	IoQueues              string `map:"ioQueues"`
	KeepAliVeTimeout      string `map:"ioQueues"`
	ControllerLossTimeout string `map:"controllerLossTimeoutSec"`
	ReconnectDelay        string `map:"reconnectDelaySec"`
}

func New(conf blockcon.CommonConfig) (blockcon.Manager, error) {
	result := &connector{}
	result.commonConfig = conf

	err := conf.Cfg.Decode(&result.config)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	result.config.FillDefaults()
	if err := result.config.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	result.snip = newNvmeSnippets(conf.Tr, result.config.NvmeCmd, conf.DevFolder, result.config.ParseKernelLog)

	return result, nil
}

func (c *connector) Capabilities(ctx context.Context) (blockcon.Capabilities, error) {
	return blockcon.Capabilities{
		Protocol:    blockapi.Nvme,
		HasNodeInfo: false,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req blockcon.ConnectRequest) (blockcon.Handle, error) {
	proto, ok := req.Proto.(blockapi.NvmeShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch")
	}

	var parsedPreset PresetData
	err := serialmaps.Parse(req.Preset, mapPrefix, &parsedPreset)
	if err != nil {
		return nil, fmt.Errorf("parse preset: %w", err)
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
		IoQueues:              parsedPreset.IoQueues,
		KeepAliveTimeout:      parsedPreset.KeepAliVeTimeout,
		ControllerLossTimeout: parsedPreset.ControllerLossTimeout,
		ReconnectDelay:        parsedPreset.ReconnectDelay,
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

func (c *connector) ParseHandle(serializedConnectorData string) (blockcon.Handle, error) {
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

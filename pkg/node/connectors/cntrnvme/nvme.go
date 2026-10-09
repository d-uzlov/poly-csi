package cntrnvme

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/protocols"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type connector struct {
	connectors.NoInfo

	commonConfig connectors.CommonConfig
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

func New(conf connectors.CommonConfig) (connectors.Manager, error) {
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

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		Protocol:    protocols.Nvme,
		HasNodeInfo: false,
		ConnectType: connectors.ConnectTypeBlock,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req connectors.ConnectRequest) (connectors.Handle, error) {
	pc, err := protocols.Parse[protocols.NvmeShare](req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("protocols.Parse: %w", err)
	}

	var parsedPreset PresetData
	err = serialmaps.Parse(req.Preset, mapPrefix, &parsedPreset)
	if err != nil {
		return nil, fmt.Errorf("parse preset: %w", err)
	}

	cp := ConnectParams{
		TargetNqn:     pc.TargetNqn,
		HostNqn:       pc.HostNqn,
		HostId:        pc.HostId,
		Port:          pc.ConnectPort,
		NvmeTransport: pc.TransportType,
	}
	var auth *ConnectAuth
	if pc.HostKey != "" && pc.ControllerKey != "" {
		auth = &ConnectAuth{
			HostKey:       pc.HostKey,
			ControllerKey: pc.ControllerKey,
		}
	}
	extra := ConnectExtra{
		IoQueues:              parsedPreset.IoQueues,
		KeepAliveTimeout:      parsedPreset.KeepAliVeTimeout,
		ControllerLossTimeout: parsedPreset.ControllerLossTimeout,
		ReconnectDelay:        parsedPreset.ReconnectDelay,
	}

	ecs := []ExpectedConnection{}
	for _, ao := range req.AddressOptions {
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

func (c *connector) ParseHandle(serializedConnectorData string) (connectors.Handle, error) {
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

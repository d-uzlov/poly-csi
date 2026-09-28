package cntrnvme

import (
	"context"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type connector struct {
	commonConfig connectors.CommonConfig
	snip         NvmeSnippets
	config       Config
}

const mapPrefix = "nvme."

type PresetData struct {
	IoQueues              string `map:"ioQueues"`
	KeepAliVeTimeout      string `map:"ioQueues"`
	ControllerLossTimeout string `map:"controllerLossTimeoutSec"`
	ReconnectDelay        string `map:"reconnectDelaySec"`
}

func New() connectors.Connector {
	return &connector{}
}

func (c *connector) Configure(ctx context.Context, conf connectors.CommonConfig, cfg yaml.Node) error {
	c.commonConfig = conf

	err := cfg.Decode(&c.config)
	if err != nil{
		return fmt.Errorf("parse config: %w", err)
	}
	c.config.FillDefaults()
	if err := c.config.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	c.snip = newNvmeSnippets(conf.Tr, c.config.NvmeCmd, conf.DevFolder, c.config.ParseKernelLog)

	return nil
}

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		ConnectType: connectors.ConnectTypeBlock,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req connectors.ConnectRequest) (connectors.ConnectionHandle, error) {
	pc, sc, err := ParsePublishContext(req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	err = validateContext(pc, sc)
	if err != nil {
		return nil, fmt.Errorf("validateContext: %w", err)
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
	if sc.HostKey != "" && sc.ControllerKey != "" {
		auth = &ConnectAuth{
			HostKey:       sc.HostKey,
			ControllerKey: sc.ControllerKey,
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

func (c *connector) ParseHandle(serializedConnectorData string) (connectors.ConnectionHandle, error) {
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

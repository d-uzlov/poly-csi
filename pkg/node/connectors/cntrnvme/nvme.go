package cntrnvme

import (
	"context"
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
)

type connector struct {
	commonConfig connectors.CommonConfig
	snip         NvmeSnippets
	config       Config

	ioQueues              string
	keepAliveTimeout      string
	controllerLossTimeout string
	reconnectDelay        string
}

const ioQueuesKey = "nvme.ioQueues"
const keepAliveTimeoutKey = "nvme.keepAliveTimeoutSec"
const controllerLossTimeoutKey = "nvme.controllerLossTimeoutSec"
const reconnectDelayKey = "nvme.reconnectDelaySec"

func New() connectors.Connector {
	return &connector{}
}

func (c *connector) Configure(ctx context.Context, conf connectors.CommonConfig, opaqueConfig []byte) error {
	c.commonConfig = conf

	if err := yaml.Unmarshal(opaqueConfig, &c.config); err != nil {
		return fmt.Errorf("parse opaqueConfig: %w", err)
	}
	c.config.FillDefaults()
	if err := c.config.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	c.snip = newNvmeSnippets(conf.Tr, c.config.NvmeCmd, conf.DevFolder, c.config.ParseKernelLog)
	c.ioQueues = conf.Preset[ioQueuesKey]
	c.keepAliveTimeout = conf.Preset[keepAliveTimeoutKey]
	c.controllerLossTimeout = conf.Preset[controllerLossTimeoutKey]
	c.reconnectDelay = conf.Preset[reconnectDelayKey]

	return nil
}

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		ConnectType: connectors.ConnectTypeBlock,
	}, nil
}

func (c *connector) Connect(ctx context.Context, req connectors.ConnectRequest) (connectors.State, error) {
	pc, sc, err := ParsePublishContext(req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	err = validateContext(pc, sc)
	if err != nil {
		return nil, fmt.Errorf("validateContext: %w", err)
	}

	successCount := 0
	for _, v := range req.AddressOptions {
		err = c.snip.Connect(ctx, pc.TargetNqn, pc.HostNqn, pc.HostId, pc.ConnectPort, pc.TransportType, v.Address, c.ioQueues, sc.HostKey, sc.ControllerKey, c.keepAliveTimeout, c.controllerLossTimeout, c.reconnectDelay)
		if err != nil {
			if len(req.AddressOptions) <= 1 {
				return nil, fmt.Errorf("nvmeConnect: %w", err)
			}

			logs.FromCtx(ctx).Error("nvmeConnect failed", "address", v, "error", err.Error())
			continue
		}
		successCount++
	}
	if successCount == 0 {
		return nil, fmt.Errorf("all %d nvme multipath connections failed, see logs for details", len(req.AddressOptions))
	}

	return &state{
		con: c,
		Nqn: pc.TargetNqn,
	}, nil
}

func (c *connector) ParseState(serializedConnectorData string) (connectors.State, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.con = c

	return res, nil
}

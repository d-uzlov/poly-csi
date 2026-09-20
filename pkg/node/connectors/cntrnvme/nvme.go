package cntrnvme

import (
	"context"
	"fmt"
	"strings"

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

	expectedConnections := []ExpectedConnection{}
	successCount := 0
	for _, v := range req.AddressOptions {
		ec := ExpectedConnection{
			AddrName: v.Name,
		}
		expectedConnections = append(expectedConnections, ec)
		controllerName, err := c.snip.Connect(ctx, pc.TargetNqn, pc.HostNqn, pc.HostId, pc.ConnectPort, pc.TransportType, v.Address, c.ioQueues, sc.HostKey, sc.ControllerKey, c.keepAliveTimeout, c.controllerLossTimeout, c.reconnectDelay)
		if err != nil {
			if len(req.AddressOptions) <= 1 {
				return nil, fmt.Errorf("nvmeConnect: %w", err)
			}

			logs.FromCtx(ctx).Error("nvmeConnect failed", "address", v, "error", err.Error())
			continue
		}
		ec.Controller = controllerName
		successCount++
	}
	if successCount == 0 {
		return nil, fmt.Errorf("all %d nvme multipath connections failed, see logs for details", len(req.AddressOptions))
	}

	subsys, err := c.snip.GetSubsysInfo(ctx, pc.TargetNqn)
	if err != nil {
		return nil, fmt.Errorf("GetSubsysInfo: %w", err)
	}
	if subsys == nil {
		return nil, fmt.Errorf("subsys not found: %s", pc.TargetNqn)
	}

	if len(subsys.Namespaces) == 0 {
		return nil, fmt.Errorf("subsystem does not have any namespaces")
	}
	var namespaceDevice string
	for _, ns := range subsys.Namespaces {
		if ns.NSID == 1 {
			namespaceDevice = ns.NameSpace
			break
		}
	}
	if namespaceDevice == "" {
		return nil, fmt.Errorf("could not find namespace block device")
	}

	serial := subsys.Controllers[0].SerialNumber
	for _, v := range subsys.Controllers {
		if serial != v.SerialNumber {
			return nil, fmt.Errorf("different controllers have different serial number")
		}
		for i, ec := range expectedConnections {
			if ec.Controller == v.Controller {
				expectedConnections[i].AddrIp = parseNvmeAddress(v.Address)
			}
		}
	}

	return &state{
		con:         c,
		Nqn:         pc.TargetNqn,
		DeviceName:  namespaceDevice,
		Serial:      serial,
		Connections: expectedConnections,
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

func parseNvmeAddress(full string) string {
	for v := range strings.SplitSeq(full, ",") {
		if suff, ok := strings.CutPrefix(v, "traddr="); ok {
			return suff
		}
	}
	return ""
}

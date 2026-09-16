package cntrhttpjsonplugin

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/node/connectors"
)

type DispatcherClient struct {
	remote Sender
	conf   connectors.CommonConfig
}

var _ connectors.Connector = &DispatcherClient{}

func NewDispatcherClient(sender Sender) *DispatcherClient {
	return &DispatcherClient{
		remote: sender,
	}
}

func (c *DispatcherClient) Configure(ctx context.Context, conf connectors.CommonConfig, opaqueConfig []byte) error {
	c.conf = conf
	c.remote.SetConfig(opaqueConfig)
	return nil
}

func (c *DispatcherClient) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	req := PluginRequest{
		Method:        MethodGetCapabilities,
		Preset:        c.conf.Preset,
		State:         "",
		SerializedArg: "",
	}
	resp, err := c.remote.Send(ctx, req)
	if err != nil {
		return connectors.Capabilities{}, err
	}

	var res connectors.Capabilities
	err = json.Unmarshal([]byte(resp), &res)
	if err != nil {
		return connectors.Capabilities{}, err
	}

	return res, nil
}

func (c *DispatcherClient) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	req := PluginRequest{
		Method:        MethodGetInfo,
		Preset:        c.conf.Preset,
		State:         "",
		SerializedArg: "",
	}
	resp, err := c.remote.Send(ctx, req)
	if err != nil {
		return nil, err
	}

	var res connectors.ConnectorNodeInfo
	err = json.Unmarshal([]byte(resp), &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (c *DispatcherClient) Connect(ctx context.Context, cReq connectors.ConnectRequest) (connectors.State, error) {
	serializedArg, err := json.Marshal(cReq)
	if err != nil {
		return nil, fmt.Errorf("serialize connect request: %w", err)
	}
	req := PluginRequest{
		Method:        MethodConnect,
		Preset:        c.conf.Preset,
		State:         "",
		SerializedArg: string(serializedArg),
	}
	resp, err := c.remote.Send(ctx, req)
	if err != nil {
		return nil, err
	}

	return &state{
		con:         c,
		RemoteState: resp,
	}, nil
}

func (c *DispatcherClient) ParseState(serializedConnectorData string) (connectors.State, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.con = c

	return res, nil
}

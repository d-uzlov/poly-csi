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
	cc     clientConfig
}

var _ connectors.Manager = &DispatcherClient{}

type clientConfig struct {
	RemoteId string `yaml:"remoteId"`
}

func NewDispatcherClient(sender Sender, conf connectors.CommonConfig) (connectors.Manager, error) {
	err := sender.SetConfig(conf.Cfg)
	if err != nil {
		return nil, err
	}
	var cc clientConfig
	err = conf.Cfg.Decode(&cc)
	if err != nil {
		return nil, err
	}
	if cc.RemoteId == "" {
		return nil, fmt.Errorf("missing RemoteId")
	}
	return &DispatcherClient{
		remote: sender,
		conf:   conf,
		cc:     cc,
	}, nil
}

func (c *DispatcherClient) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	req := PluginRequest{
		Method:        MethodGetCapabilities,
		ConnectorId:   c.cc.RemoteId,
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
		ConnectorId:   c.cc.RemoteId,
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

func (c *DispatcherClient) GetHandle(ctx context.Context, cReq connectors.ConnectRequest) (connectors.Handle, error) {
	serializedArg, err := json.Marshal(cReq)
	if err != nil {
		return nil, fmt.Errorf("serialize connect request: %w", err)
	}
	req := PluginRequest{
		Method:        MethodMakeHandle,
		ConnectorId:   c.cc.RemoteId,
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

func (c *DispatcherClient) ParseHandle(serializedConnectorData string) (connectors.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.con = c

	return res, nil
}

package blockclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"reflect"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/blockpluginapi"
	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/api/publishcontext"
	"meoe.io/poly-csi/pkg/utils/simplerpc"
)

type DispatcherClient struct {
	client  http.Client
	address string
	conf    blockapi.CommonConfig
	cc      clientConfig
}

var _ blockapi.Manager = &DispatcherClient{}

type clientConfig struct {
	RemoteId   string `yaml:"remoteId"`
	SocketPath string `yaml:"socketPath"`
}

func New(conf blockapi.CommonConfig) (blockapi.Manager, error) {
	// err := sender.SetConfig(conf.Cfg)
	// if err != nil {
	// 	return nil, err
	// }
	var cc clientConfig
	err := conf.Cfg.Decode(&cc)
	if err != nil {
		return nil, err
	}
	if cc.RemoteId == "" {
		return nil, fmt.Errorf("missing RemoteId")
	}

	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", cc.SocketPath)
			},
		},
	}
	return &DispatcherClient{
		client:  httpClient,
		address: "http://unix",
		conf:    conf,
		cc:      cc,
	}, nil
}

func (c *DispatcherClient) Capabilities(ctx context.Context) (blockapi.Capabilities, error) {
	var result blockpluginapi.GetCapabilitiesResponse
	err := simplerpc.MakeQuery(ctx, c.client, c.address+blockpluginapi.Capabilities, blockpluginapi.InvokeRequest{
		ConnectorId: c.cc.RemoteId,
	}, &result)
	if err != nil {
		return blockapi.Capabilities{}, err
	}

	return result.Caps, nil
}

func (c *DispatcherClient) GetInfo(ctx context.Context) (blockspec.NodeInfo, error) {
	return nil, fmt.Errorf("plugin does not support get info")
}

func (c *DispatcherClient) GetHandle(ctx context.Context, cReq blockapi.ConnectRequest) (blockapi.Handle, error) {
	protoString, err := json.Marshal(cReq.Proto)
	if err != nil {
		return nil, fmt.Errorf("serialize protocol data: %w", err)
	}
	protoName, err := publishcontext.Registry.NameOf(reflect.ValueOf(cReq.Proto))
	if err != nil {
		return nil, fmt.Errorf("get name of protocol: %w", err)
	}
	cReq.Proto = nil
	var result blockpluginapi.InvokeStateResponse
	err = simplerpc.MakeQuery(ctx, c.client, c.address+blockpluginapi.GetHandle, blockpluginapi.GetHandleRequest{
		ConnectorId: c.cc.RemoteId,
		Data:        cReq,
		ProtoName:   protoName,
		Protocol:    protoString,
	}, &result)
	if err != nil {
		return nil, err
	}
	if result.ErrorString != "" {
		return nil, fmt.Errorf("remote error: %s", result.ErrorString)
	}

	return &state{
		con:         c,
		RemoteState: result.State,
	}, nil
}

func (c *DispatcherClient) ParseHandle(serializedConnectorData json.RawMessage) (blockapi.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.con = c

	return res, nil
}

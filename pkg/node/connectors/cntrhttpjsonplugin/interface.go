package cntrhttpjsonplugin

import (
	"context"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/logs"
)

type Sender interface {
	SetConfig(cfg yaml.Node) error
	Send(ctx context.Context, req PluginRequest) (result string, state string, err error)
}

type PluginRequest struct {
	Method        string `json:"method"`
	State         string `json:"state"`
	ConnectorId   string `json:"connectorId"`
	SerializedArg string `json:"serializedArg"`
}

type PluginResponse struct {
	Status string            `json:"status"`
	Err    string            `json:"error"`
	State  string            `json:"state"`
	Result string            `json:"result"`
	Logs   []logs.LogRequest `json:"logs"`
}

const ResponseStatusError = "error"
const ResponseStatusOk = "ok"

// connector methods
const (
	MethodGetCapabilities = "/capabilities"
	MethodGetInfo         = "/get_info"
	MethodMakeHandle      = "/make_handle"
)

// state methods
const (
	MethodConnectBlock  = "/connect_block"
	MethodConnectFolder = "/connect_folder"
	MethodGetDeviceList = "/get_device_list"
	MethodDisconnect    = "/disconnect"
	MethodExpand        = "/expand"
)

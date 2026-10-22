package blockpluginapi

import (
	"encoding/json"

	"meoe.io/poly-csi/pkg/api/blockapi"
)

type InvokeRequest struct {
	ConnectorId string `json:"connectorId"`
}

type InvokeResponse struct {
	ErrorString string `json:"errorString,omitempty"`
}

type GetCapabilitiesResponse struct {
	InvokeResponse
	Caps blockapi.Capabilities `json:"caps"`
}

type GetHandleRequest struct {
	InvokeRequest
	Data      blockapi.ConnectRequest `json:"data"`
	ProtoName string                  `json:"protocolName"`
	Protocol  json.RawMessage         `json:"protocol"`
}

type InvokeStateRequest struct {
	InvokeRequest
	State json.RawMessage `json:"state"`
}

type InvokeStateResponse struct {
	InvokeResponse
	State json.RawMessage `json:"state"`
}

type GetDevicesResponse struct {
	InvokeStateResponse
	Devices []string `json:"devices"`
}

type ResponseProvider interface {
	Get() InvokeResponse
}

func (r InvokeResponse) Get() InvokeResponse {
	return r
}

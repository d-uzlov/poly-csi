package blockclient

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/blockpluginapi"
	"meoe.io/poly-csi/pkg/utils/simplerpc"
)

type state struct {
	con *DispatcherClient `json:"-"`

	RemoteState json.RawMessage `json:"remoteState"`
}

var _ blockapi.Handle = &state{}

func parseState(serialized json.RawMessage) (*state, error) {
	var result state
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *state) Connect(ctx context.Context) error {
	var result blockpluginapi.InvokeStateResponse
	err := simplerpc.MakeQuery(ctx, s.con.client, s.con.address+blockpluginapi.StateConnect, blockpluginapi.InvokeStateRequest{
		ConnectorId: s.con.cc.RemoteId,
		State:       s.RemoteState,
	}, &result)
	if err != nil {
		return err
	}
	s.RemoteState = result.State
	if result.ErrorString != "" {
		return fmt.Errorf("remote error: %s", result.ErrorString)
	}

	return nil
}

func (s *state) GetDevices(ctx context.Context) ([]string, error) {
	var result blockpluginapi.GetDevicesResponse
	err := simplerpc.MakeQuery(ctx, s.con.client, s.con.address+blockpluginapi.StateGetDevices, blockpluginapi.InvokeStateRequest{
		ConnectorId: s.con.cc.RemoteId,
		State:       s.RemoteState,
	}, &result)
	if err != nil {
		return nil, err
	}
	s.RemoteState = result.State
	if result.ErrorString != "" {
		return nil, fmt.Errorf("remote error: %s", result.ErrorString)
	}

	// server strips /dev equivalent prefix, so we need to append it back
	for k, v := range result.Devices {
		if v[0] != '/' {
			v = "/" + v
		}
		result.Devices[k] = s.con.conf.DevFolder + v
	}

	return result.Devices, nil
}

func (s *state) Disconnect(ctx context.Context) error {
	var result blockpluginapi.InvokeStateResponse
	err := simplerpc.MakeQuery(ctx, s.con.client, s.con.address+blockpluginapi.StateDisconnect, blockpluginapi.InvokeStateRequest{
		ConnectorId: s.con.cc.RemoteId,
		State:       s.RemoteState,
	}, &result)
	if err != nil {
		return err
	}
	s.RemoteState = result.State
	if result.ErrorString != "" {
		return fmt.Errorf("remote error: %s", result.ErrorString)
	}

	return nil
}

func (s *state) Expand(ctx context.Context) error {
	var result blockpluginapi.InvokeStateResponse
	err := simplerpc.MakeQuery(ctx, s.con.client, s.con.address+blockpluginapi.StateExpand, blockpluginapi.InvokeStateRequest{
		ConnectorId: s.con.cc.RemoteId,
		State:       s.RemoteState,
	}, &result)
	if err != nil {
		return err
	}
	s.RemoteState = result.State
	if result.ErrorString != "" {
		return fmt.Errorf("remote error: %s", result.ErrorString)
	}

	return nil
}

func (s *state) CheckHealth(ctx context.Context) ([]blockapi.HealthMessage, error) {
	return nil, nil
}

func (s *state) Serialize() (json.RawMessage, error) {
	res, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *state) Validate() error {
	if s.con == nil {
		return fmt.Errorf("parent connector is not set")
	}
	if s.RemoteState == nil {
		return fmt.Errorf("missing RemoteState")
	}
	return nil
}

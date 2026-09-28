package cntrhttpjsonplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"meoe.io/poly-csi/pkg/node/connectors"
)

type state struct {
	con *DispatcherClient `json:"-"`

	RemoteState string `json:"remoteState"`
}

var _ connectors.Handle = &state{}

func parseState(serialized string) (*state, error) {
	var result state
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *state) ConnectBlock(ctx context.Context) error {
	req := PluginRequest{
		Method:        MethodConnectBlock,
		ConnectorId:   s.con.cc.RemoteId,
		State:         s.RemoteState,
		SerializedArg: "",
	}
	_, err := s.con.remote.Send(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (s *state) ConnectFolder(ctx context.Context, cReq connectors.ConnectFolderRequest) error {
	serializedArg, err := json.Marshal(cReq)
	if err != nil {
		return fmt.Errorf("serialize connect request: %w", err)
	}
	req := PluginRequest{
		Method:        MethodConnectFolder,
		ConnectorId:   s.con.cc.RemoteId,
		State:         s.RemoteState,
		SerializedArg: string(serializedArg),
	}
	_, err = s.con.remote.Send(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (s *state) GetDevices(ctx context.Context, index int) ([]string, error) {
	req := PluginRequest{
		Method:        MethodGetDeviceList,
		ConnectorId:   s.con.cc.RemoteId,
		State:         s.RemoteState,
		SerializedArg: strconv.Itoa(index),
	}
	resp, err := s.con.remote.Send(ctx, req)
	if err != nil {
		return nil, err
	}

	var res []string
	err = json.Unmarshal([]byte(resp), &res)
	if err != nil {
		return nil, err
	}

	// server strips /dev equivalent prefix, so we need to append it back
	for k, v := range res {
		if v[0] != '/' {
			v = "/" + v
		}
		res[k] = s.con.conf.DevFolder + v
	}

	return res, nil
}

func (s *state) Disconnect(ctx context.Context) error {
	req := PluginRequest{
		Method:        MethodDisconnect,
		ConnectorId:   s.con.cc.RemoteId,
		State:         s.RemoteState,
		SerializedArg: "",
	}
	_, err := s.con.remote.Send(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (s *state) Expand(ctx context.Context) error {
	req := PluginRequest{
		Method:        MethodExpand,
		ConnectorId:   s.con.cc.RemoteId,
		State:         s.RemoteState,
		SerializedArg: "",
	}
	_, err := s.con.remote.Send(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (s *state) CheckHealth(ctx context.Context) ([]connectors.HealthMessage, error) {
	return nil, nil
}

func (s *state) Serialize() (string, error) {
	res, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func (s *state) Validate() error {
	if s.con == nil {
		return fmt.Errorf("parent connector is not set")
	}
	if s.RemoteState == "" {
		return fmt.Errorf("missing RemoteState")
	}
	return nil
}

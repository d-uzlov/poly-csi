package cntrhttpjsonplugin

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/node/connectors"
)

type state struct {
	con *DispatcherClient

	RemoteState string `json:"remoteState"`
}

var _ connectors.State = &state{}

func parseState(serialized string) (*state, error) {
	var result state
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, err
	}
	err = result.Validate()
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *state) GetDevices(ctx context.Context) ([]string, error) {
	req := PluginRequest{
		Method:        MethodGetDeviceList,
		Preset:        s.con.conf.Preset,
		State:         s.RemoteState,
		SerializedArg: "",
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
		Preset:        s.con.conf.Preset,
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
		Preset:        s.con.conf.Preset,
		State:         s.RemoteState,
		SerializedArg: "",
	}
	_, err := s.con.remote.Send(ctx, req)
	if err != nil {
		return err
	}

	return nil
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

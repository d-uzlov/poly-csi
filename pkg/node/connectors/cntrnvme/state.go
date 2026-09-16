package cntrnvme

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/node/connectors"
)

type state struct {
	con *connector

	Nqn string `json:"nqn"`
}

var _ connectors.State = &state{}

func parseState(serialized string) (*state, error) {
	var result state
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *state) GetDevices(ctx context.Context) ([]string, error) {

	namespaceDevice, err := s.con.snip.FindNamespaceDevice(ctx, s.Nqn, 1)
	if err != nil {
		return nil, fmt.Errorf("nvmeFindNamespace: %w", err)
	}

	namespaceDevicePath := s.con.commonConfig.DevFolder + "/" + namespaceDevice
	return []string{namespaceDevicePath}, nil
}

func (s *state) Disconnect(ctx context.Context) error {
	err := s.con.snip.Disconnect(ctx, s.Nqn)
	if err != nil {
		return fmt.Errorf("nvmeDisconnect: %w", err)
	}
	return nil
}

func (s *state) Expand(ctx context.Context) error {
	err := s.con.snip.Rescan(ctx, s.Nqn)
	if err != nil {
		return fmt.Errorf("nvmeRescan: %w", err)
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
	if s.Nqn == "" {
		return fmt.Errorf("missing nqn")
	}
	return nil
}

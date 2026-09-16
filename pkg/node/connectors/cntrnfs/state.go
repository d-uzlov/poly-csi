package cntrnfs

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/node/connectors"
)

type state struct {
	con *connector

	Path string `map:"nfs.share_path"`
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
	return nil, fmt.Errorf("nfs does not create block devices")
}

func (s *state) Disconnect(ctx context.Context) error {
	// unmount is managed externally
	return nil
}

func (s *state) Expand(ctx context.Context) error {
	// nfs volumes don't need expanding
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
	if s.Path == "" {
		return fmt.Errorf("missing share path")
	}
	return nil
}

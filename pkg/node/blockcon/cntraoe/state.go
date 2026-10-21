package cntraoe

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/node/blockcon"
	"meoe.io/poly-csi/pkg/transport"
)

type state struct {
	tr        transport.Transport `json:"-"`
	devFolder string              `json:"-"`

	Pc blockspec.AoeShare `json:"pc"`
}

var _ blockcon.Handle = &state{}

func parseState(serialized string) (*state, error) {
	var result state
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *state) getDevName() string {
	return fmt.Sprintf("e%d.%d", *s.Pc.Shelf, *s.Pc.Slot)
}

func (s *state) Connect(ctx context.Context) error {
	snip := NewSnippets(s.tr)
	return snip.Discover(ctx)
}

func (s *state) GetDevices(ctx context.Context) ([]string, error) {
	return []string{fmt.Sprintf("%s/etherd/%s", s.devFolder, s.getDevName())}, nil
}

func (s *state) Disconnect(ctx context.Context) error {
	snip := NewSnippets(s.tr)
	return snip.FlushDevice(ctx, s.getDevName())
}

func (s *state) Expand(ctx context.Context) error {
	snip := NewSnippets(s.tr)
	return snip.RescanDevice(ctx, s.getDevName())
}

func (s *state) CheckHealth(ctx context.Context) ([]blockcon.HealthMessage, error) {
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
	if s.tr == nil {
		return fmt.Errorf("missing transport")
	}
	if s.devFolder == "" {
		return fmt.Errorf("missing DevFolder")
	}
	return nil
}

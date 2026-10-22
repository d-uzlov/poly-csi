package cntrublkiscsi

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/node/blockcon/cntrublk"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/ublk"
)

type state struct {
	tr        transport.Transport `json:"-"`
	devFolder string              `json:"-"`
	sv        cntrublk.Supervisor `json:"-"`

	Uuid    string               `json:"uuid"`
	Address string               `json:"address"`
	Pc      blockspec.IscsiShare `json:"iscsi"`

	IscsiUrl string `json:"IscsiUrl"`
	DevId    string `json:"devId"`
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

func (s *state) Connect(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			s.sv.Remove(s.Uuid)
			s.DevId = ""
		}
	}()

	url := ublk.IscsiUrlParams{
		TargetIqn: s.Pc.Iqn,
		Host:      s.Address,
		Port:      s.Pc.Port,
		Lun:       "0", // TODO add lun into iscsi share
	}
	switch s.Pc.AuthType {
	case blockspec.IscsiAuthNone:
		// nothing to do
	case blockspec.IscsiAuthChap:
		url.Username = s.Pc.ClientUser
		url.Password = s.Pc.ClientPass
		url.TargetUsername = s.Pc.TargetUser
		url.TargetPassword = s.Pc.TargetPass
	}
	err = url.Validate()
	if err != nil {
		return fmt.Errorf("iscsi url: %w", err)
	}
	s.IscsiUrl = url.String()

	info, err := s.sv.Launch(ctx, s.Uuid, ublk.CreateParams{
		Type:     "iscsi",
		Zerocopy: false,
	}, &ublk.CreateIscsi{
		InitiatorName: s.Pc.Initiator,
		Url:           s.IscsiUrl,
	})
	if err != nil {
		return fmt.Errorf("supervisor.Launch: %w", err)
	}

	s.DevId = info.DevId

	if !info.Flags.F_user_recovery {
		return fmt.Errorf("created device devId=%s is missing recovery flag", info.DevId)
	}

	return nil
}

func (s *state) GetDevices(ctx context.Context) ([]string, error) {
	if s.DevId == "" {
		return nil, fmt.Errorf("missing s.DevId")
	}
	return []string{s.devFolder + "/ublkb" + s.DevId}, nil
}

func (s *state) Disconnect(ctx context.Context) error {
	return s.sv.Remove(s.Uuid)
}

func (s *state) Expand(ctx context.Context) error {
	// TODO
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
	if s.tr == nil {
		return fmt.Errorf("missing transport")
	}
	if s.sv == nil {
		return fmt.Errorf("missing supervisor")
	}
	if s.devFolder == "" {
		return fmt.Errorf("missing devFolder")
	}
	if s.Address == "" {
		return fmt.Errorf("missing Address")
	}
	return nil
}

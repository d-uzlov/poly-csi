package cntrublknbd

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrublk"
	"meoe.io/poly-csi/pkg/protocols"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/ublk"
)

type state struct {
	tr        transport.Transport   `json:"-"`
	devFolder string                `json:"-"`
	callback  cntrublk.UblkCallback `json:"-"`

	Address string             `json:"address"`
	Pc      protocols.NbdShare `json:"nbd"`

	DevId string `json:"devId"`
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

func (s *state) ConnectBlock(ctx context.Context) (err error) {
	snip := ublk.New(s.tr)

	// ublk will happily create several devices connecting to the same target
	// so we need to check if any of existing devices is the one we need
	list, err := snip.List(ctx, "")
	if err != nil {
		return fmt.Errorf("listDevices: %w", err)
	}
	for _, v := range list {
		if v.Target == "" {
			continue
		}
		if v.TargetType != "nbd" {
			continue
		}
		target, err := v.ParseTarget[ublk.NbdTarget]()
		if err != nil {
			logs.FromCtx(ctx).Warn("failed to parse ublk target from json", "devId", v.DevId, "target", v.Target, "error", err)
			continue
		}
		if s.checkSameTarget(target) {
			logs.FromCtx(ctx).Info("found existing matching target", "devId", v.DevId)
			s.DevId = v.DevId
			return nil
		}
	}

	info, err := snip.Create(ctx, ublk.CreateParams{
		Type:     "nbd",
		Zerocopy: true,
	}, ublk.CreateNbd{
		Host:         s.Address,
		Port:         s.Pc.Port,
		ExportName:   s.Pc.ExportName,
		SendZeroCopy: true,
	})
	if err != nil {
		return fmt.Errorf("snip.Create: %w", err)
	}

	s.DevId = info.DevId

	if !info.Flags.F_user_recovery {
		return fmt.Errorf("created device devId=%s is missing recovery flag", info.DevId)
	}

	if s.callback != nil {
		s.callback.Add(info.DevId, info.DaemonPid)
	}

	return nil
}

func (s *state) ConnectFolder(ctx context.Context, req connectors.ConnectFolderRequest) error {
	return fmt.Errorf("nbd: ConnectFolder is not implemented")
}

func (s *state) GetDevices(ctx context.Context, index int) ([]string, error) {
	return []string{s.devFolder + "/ublkb" + s.DevId}, nil
}

func (s *state) Disconnect(ctx context.Context) error {
	// TODO is it safe to call callback before we are sure devId is correct?
	if s.callback != nil {
		s.callback.Remove(s.DevId)
	}
	snip := ublk.New(s.tr)
	list, err := snip.List(ctx, s.DevId)
	if err != nil {
		return fmt.Errorf("list existing devices before deletion: %w", err)
	}
	if len(list) == 0 {
		// device with devId is already deleted
		return nil
	}
	if len(list) > 1 {
		return fmt.Errorf("snip.List(devId=%s) returned several results", s.DevId)
	}
	info := list[0]
	if info.DevId != s.DevId {
		return fmt.Errorf("snip.List(devId=%s) returned info for devId=%s", s.DevId, info.DevId)
	}
	if info.TargetType != "nbd" {
		return nil
	}
	if info.Target == "" {
		// maybe someone else has created device with same devId
		// it's not safe to delete without detailed target info
		logs.FromCtx(ctx).Warn("tried to delete ublk device without target info, skipping", "devId", s.DevId)
		return nil
	}

	target, err := info.ParseTarget[ublk.NbdTarget]()
	if err != nil {
		return err
	}
	if !s.checkSameTarget(target) {
		return nil
	}
	return snip.Delete(ctx, s.DevId)
}

func (s *state) Expand(ctx context.Context) error {
	// TODO
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
	if s.tr == nil {
		return fmt.Errorf("missing transport")
	}
	if s.Address == "" {
		return fmt.Errorf("missing Address")
	}
	return nil
}

func (s *state) checkSameTarget(target ublk.NbdTarget) bool {
	return target.Host == s.Address && target.Port == s.Pc.Port && target.ExportName == s.Pc.ExportName
}

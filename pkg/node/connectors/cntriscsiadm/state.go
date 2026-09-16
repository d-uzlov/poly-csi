package cntriscsiadm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type state struct {
	con *connector

	Iqn string `json:"iqn"`
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
	tr := s.con.commonConfig.Tr
	snip := newIscsiSnippets(tr, ctx, s.con.config.IscsiadmCmd)

	is, err := snip.findSessions(s.Iqn)
	if err != nil {
		return nil, fmt.Errorf("find iscsi sessions: %w", err)
	}

	logs.FromCtx(ctx).MicroOp("iscsi: find device paths")

	var combinedError string
	var devices []string
	for _, v := range is {
		// /sys/class/iscsi_session/sessionX/state
		// /sys/class/iscsi_connection/connectionX\:0/state
		statePath := fmt.Sprintf("/sys/class/iscsi_session/session%s/state", v.index)
		err = transport.Retry(time.Millisecond*100, time.Second, func() error {
			stateBytes, err := tr.Cmds().Read(ctx, statePath, true)
			if err != nil {
				// runtime errors should be self-explanatory
				return err
			}
			state := string(stateBytes)
			if state != "LOGGED_IN" {
				return fmt.Errorf("unexpected iscsi session state: %s", state)
			}
			return nil
		})
		if err != nil {
			combinedError += fmt.Sprintf("iscsi device for %s is not online: %s ; ", v.address, err.Error())
			logs.FromCtx(ctx).Error("iscsi device is not online", "address", v.address, "error", err.Error())
			continue
		}

		deviceLink := fmt.Sprintf(s.con.commonConfig.DevFolder+"/disk/by-path/ip-%s-iscsi-%s-lun-0", v.address, v.iqn)
		// iscsi creates links in background, need to wait for it to appear
		err = transport.Retry(time.Millisecond*100, time.Second, func() error {
			stat, err := tr.Cmds().Stat(ctx, deviceLink)
			if err != nil {
				// runtime errors should be self-explanatory
				return err
			}
			if !stat.FileExists {
				return fmt.Errorf("link does not exist: %s", deviceLink)
			}
			devices = append(devices, stat.Path)
			return nil
		})
		if err != nil {
			combinedError += fmt.Sprintf("failed to get iscsi device file link at %s: %s ; ", deviceLink, err.Error())
			logs.FromCtx(ctx).Error("failed to get iscsi device file link", "address", v.address, "path", deviceLink, "error", err.Error())
		}
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("iscsi device is not found: %s", combinedError)
	}
	// if len(devices) < len(req.AddressOptions) {
	// 	logs.FromCtx(ctx).Warn("iscsi: some addresses failed to connect, returning partial list")
	// }

	return devices, nil
}

func (s *state) Disconnect(ctx context.Context) error {
	tr := s.con.commonConfig.Tr
	snip := newIscsiSnippets(tr, ctx, s.con.config.IscsiadmCmd)

	is, err := snip.findSessions(s.Iqn)
	if err != nil {
		return fmt.Errorf("find iscsi sessions: %w", err)
	}
	for _, v := range is {
		err = snip.disconnect(s.Iqn, v.address)
		if err != nil {
			return fmt.Errorf("iscsi disconnect from %s: %w", v.address, err)
		}
	}
	return nil
}

func (s *state) Expand(ctx context.Context) error {
	tr := s.con.commonConfig.Tr
	snip := newIscsiSnippets(tr, ctx, s.con.config.IscsiadmCmd)
	// TODO rescan only target devices?
	err := snip.rescanAll()
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
	if s.Iqn == "" {
		return fmt.Errorf("missing iqn")
	}
	return nil
}

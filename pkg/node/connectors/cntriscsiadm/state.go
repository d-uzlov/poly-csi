package cntriscsiadm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/addropt"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type state struct {
	tr   transport.Transport `json:"-"`
	snip *iscsiSnippets      `json:"-"`

	DevFolder               string            `json:"devFolder"`
	DisableCleanupOnFailure bool              `json:"disableCleanupOnFailure"`
	Iqn                     string            `json:"iqn"`
	Port                    string            `json:"port"`
	Attributes              map[string]string `json:"attributes"`
	Connections             []Connection      `json:"connections"`
}

type Connection struct {
	Addr addropt.AddressOption
}

var _ connectors.ConnectionHandle = &state{}

func parseState(serialized string) (*state, error) {
	var result state
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *state) ConnectFolder(ctx context.Context, req connectors.ConnectFolderRequest) error {
	return fmt.Errorf("nvme: ConnectFolder is not implemented")
}

func (s *state) ConnectBlock(ctx context.Context) (err error) {
	// setting up cleanup before even attempting to connect
	// because failure could be caused by invalid state, that should be forcefully disconnected.
	// TODO remove this?
	defer func() {
		if s.DisableCleanupOnFailure {
			return
		}
		if err == nil {
			return
		}
		logs.FromCtx(ctx).MicroOp("detected failure, running cleanup")
		for _, v := range s.Connections {
			addr := fmt.Sprintf("%s:%s", v.Addr.Address, s.Port)
			cleanupErr := s.snip.disconnect(ctx, s.Iqn, addr)
			if cleanupErr != nil {
				logs.FromCtx(ctx).Error("failed to run cleanup", "address", addr, "error", cleanupErr.Error())
			}
		}
	}()

	successCount := 0
	for _, v := range s.Connections {
		addr := fmt.Sprintf("%s:%s", v.Addr.Address, s.Port)
		err = s.snip.configureTarget(ctx, s.Iqn, addr, s.Attributes)
		if err != nil {
			if len(s.Connections) <= 1 {
				return fmt.Errorf("iscsi: configureTarget: %w", err)
			}

			logs.FromCtx(ctx).Error("iscsi: configureTarget failed", "address", addr, "error", err.Error())
			continue
		}
		err = s.snip.connect(ctx, s.Iqn, addr)
		if err != nil {
			if len(s.Connections) <= 1 {
				return fmt.Errorf("iscsi: connect: %w", err)
			}

			logs.FromCtx(ctx).Error("iscsi: connect failed", "address", addr, "error", err.Error())
			continue
		}

		successCount++
	}
	if successCount == 0 {
		return fmt.Errorf("all %d iscsi multipath connections failed", len(s.Connections))
	}

	return nil
}

func (s *state) GetDevices(ctx context.Context) ([]string, error) {
	is, err := s.snip.findSessions(ctx, s.Iqn)
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
			stateBytes, err := s.tr.Cmds().Read(ctx, statePath, true)
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

		deviceLink := fmt.Sprintf(s.DevFolder+"/disk/by-path/ip-%s-iscsi-%s-lun-0", v.address, v.iqn)
		// iscsi creates links in background, need to wait for it to appear
		err = transport.Retry(time.Millisecond*100, time.Second, func() error {
			stat, err := s.tr.Cmds().Stat(ctx, deviceLink)
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
	is, err := s.snip.findSessions(ctx, s.Iqn)
	if err != nil {
		return fmt.Errorf("find iscsi sessions: %w", err)
	}
	for _, v := range is {
		err = s.snip.disconnect(ctx, s.Iqn, v.address)
		if err != nil {
			return fmt.Errorf("iscsi disconnect from %s: %w", v.address, err)
		}
	}
	return nil
}

func (s *state) Expand(ctx context.Context) error {
	// TODO rescan only target devices?
	err := s.snip.rescanAll(ctx)
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
	if s.tr == nil {
		return fmt.Errorf("transport is not set")
	}
	if s.snip == nil {
		return fmt.Errorf("snippets is not set")
	}
	if s.Iqn == "" {
		return fmt.Errorf("missing iqn")
	}
	if s.Attributes == nil {
		return fmt.Errorf("missing attributes")
	}
	if s.Connections == nil {
		return fmt.Errorf("missing Connections")
	}
	return nil
}

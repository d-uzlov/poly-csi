package cntrnvme

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type state struct {
	con *connector

	Nqn         string   `json:"nqn"`
	DeviceName  string   `json:"device_name"`
	Serial      string   `json:"serial"`
	Controllers []string `json:"controllers"`
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
	namespaceDevicePath := s.con.commonConfig.DevFolder + "/" + s.DeviceName
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

func (s *state) CheckHealth(ctx context.Context) ([]connectors.HealthMessage, error) {
	var issues []connectors.HealthMessage
	subsys, err := s.con.snip.GetSubsysInfo(ctx, s.Nqn)
	if err != nil {
		return nil, err
	}
	if subsys == nil {
		issues = append(issues, connectors.HealthMessage{
			Status:  connectors.HealthStatusInaccessible,
			Reason:  "no_subsys",
			Message: "subsystem is no longer connected to this host",
		})
		return issues, nil
	}

	for _, expectedController := range s.Controllers {
		found := false
		for _, controller := range subsys.Controllers {
			if expectedController == controller.Controller {
				found = true
			}
		}
		if !found {
			issues = append(issues, connectors.HealthMessage{
				Status:  connectors.HealthStatusDegraded,
				Reason:  "missing_controller",
				Message: fmt.Sprintf("controller %s: existed on initial connection but now missing", expectedController),
			})
		}
	}
	for _, controller := range subsys.Controllers {
		// can't use controller.State because sometimes it's not populated
		state, err := readState(ctx, s.con.commonConfig.Tr, controller.Controller)
		if err != nil {
			issues = append(issues, connectors.HealthMessage{
				Status:  connectors.HealthStatusDegraded,
				Reason:  "internal_error",
				Message: fmt.Sprintf("controller %s: failed to read state: %s", controller.Controller, err.Error()),
			})
		} else if state != "live" {
			issues = append(issues, connectors.HealthMessage{
				Status:  connectors.HealthStatusDegraded,
				Reason:  "controller_is_down",
				Message: fmt.Sprintf("controller %s: state is not 'live': %s", controller.Controller, state),
			})
		}
	}
	if len(subsys.Controllers) == 0 {
		// TODO can this ever happen?
		issues = append(issues, connectors.HealthMessage{
			Status:  connectors.HealthStatusInaccessible,
			Reason:  "all_controllers_down",
			Message: "subsystem is not associated with any controller",
		})
	}
	for _, ns := range subsys.Namespaces {
		cmd := append(s.con.config.NvmeCmd, "id-ns", fmt.Sprintf("%s/%s", s.con.commonConfig.DevFolder, ns.NameSpace), "--output-format=json")
		err = transport.CheckedRun(ctx, s.con.commonConfig.Tr, cmd)
		if err != nil {
			issues = append(issues, connectors.HealthMessage{
				Status:  connectors.HealthStatusInaccessible,
				Reason:  "namespace_is_down",
				Message: fmt.Sprintf("namespace %s: failed to run id-ns", ns.NameSpace),
			})
		}
	}
	if len(subsys.Namespaces) == 0 {
		issues = append(issues, connectors.HealthMessage{
			Status:  connectors.HealthStatusInaccessible,
			Reason:  "all_namespaces_down",
			Message: "subsystem does not have any namespaces",
		})
	}

	return issues, nil
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
	if s.DeviceName == "" {
		return fmt.Errorf("missing DeviceName")
	}
	if s.Serial == "" {
		return fmt.Errorf("missing Serial")
	}
	return nil
}

func readSerial(ctx context.Context, tr transport.Transport, dev string) (string, error) {
	state, err := tr.Cmds().Read(ctx, fmt.Sprintf("/sys/class/nvme/%s/serial", dev), true)
	return string(state), err
}

func readState(ctx context.Context, tr transport.Transport, dev string) (string, error) {
	state, err := tr.Cmds().Read(ctx, fmt.Sprintf("/sys/class/nvme/%s/state", dev), true)
	return string(state), err
}

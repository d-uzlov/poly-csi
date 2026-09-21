package cntrnvme

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/addropt"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type state struct {
	tr   transport.Transport
	snip *NvmeSnippets

	DevFolder   string               `json:"devFolder"`
	Params      ConnectParams        `json:"params"`
	Auth        *ConnectAuth         `json:"auth"`
	ConExtra    ConnectExtra         `json:"conExtra"`
	Connections []ExpectedConnection `json:"connections"`

	DeviceName string `json:"device_name"`
	Serial     string `json:"serial"`
}

type ExpectedConnection struct {
	Addr       addropt.AddressOption `json:"addr"`
	Controller string                `json:"controller"`
	Ip         string                `json:"ip"`
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

func (s *state) ConnectBlock(ctx context.Context) error {
	successCount := 0
	for i, v := range s.Connections {
		controllerName, err := s.snip.Connect(ctx, v.Addr.Address, s.Params, s.Auth, &s.ConExtra)
		if err != nil {
			if len(s.Connections) <= 1 {
				return fmt.Errorf("nvmeConnect: %w", err)
			}

			logs.FromCtx(ctx).Error("nvmeConnect failed", "address", v, "error", err.Error())
			continue
		}
		s.Connections[i].Controller = controllerName
		successCount++
	}
	if successCount == 0 {
		return fmt.Errorf("all %d nvme multipath connections failed, see logs for details", len(s.Connections))
	}

	subsys, err := s.snip.GetSubsysInfo(ctx, s.Params.TargetNqn)
	if err != nil {
		return fmt.Errorf("GetSubsysInfo: %w", err)
	}
	if subsys == nil {
		return fmt.Errorf("subsys not found: %s", s.Params.TargetNqn)
	}

	if len(subsys.Namespaces) == 0 {
		return fmt.Errorf("subsystem does not have any namespaces")
	}
	for _, ns := range subsys.Namespaces {
		if ns.NSID == 1 {
			s.DeviceName = ns.NameSpace
			break
		}
	}
	if s.DeviceName == "" {
		return fmt.Errorf("could not find namespace block device")
	}
	if len(subsys.Controllers) == 0 {
		return fmt.Errorf("subsystem does not have any controllers")
	}

	s.Serial = subsys.Controllers[0].SerialNumber
	for _, v := range subsys.Controllers {
		if s.Serial != v.SerialNumber {
			// TODO is this even possible?
			return fmt.Errorf("different controllers have different serial number")
		}
		for i, ec := range s.Connections {
			if ec.Controller == v.Controller {
				s.Connections[i].Ip = getIpFromNvmeAddr(v.Address)
			}
		}
	}
	return nil
}

func (s *state) GetDevices(ctx context.Context) ([]string, error) {
	namespaceDevicePath := s.DevFolder + "/" + s.DeviceName
	return []string{namespaceDevicePath}, nil
}

func (s *state) Disconnect(ctx context.Context) error {
	err := s.snip.Disconnect(ctx, s.Params.TargetNqn)
	if err != nil {
		return fmt.Errorf("nvmeDisconnect: %w", err)
	}
	return nil
}

func (s *state) Expand(ctx context.Context) error {
	err := s.snip.Rescan(ctx, s.Params.TargetNqn)
	if err != nil {
		return fmt.Errorf("nvmeRescan: %w", err)
	}
	return nil
}

func (s *state) CheckHealth(ctx context.Context) ([]connectors.HealthMessage, error) {
	var issues []connectors.HealthMessage
	subsys, err := s.snip.GetSubsysInfo(ctx, s.Params.TargetNqn)
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

	for _, ec := range s.Connections {
		found := false
		for _, controller := range subsys.Controllers {
			if ec.Controller == controller.Controller {
				found = true
			}
		}
		if !found {
			msg := fmt.Sprintf("missing controller for %s", ec.Addr.Name)
			if ec.Controller != "" {
				msg = fmt.Sprintf("missing controller for %s (was %s)", ec.Addr.Name, ec.Controller)
			}
			issues = append(issues, connectors.HealthMessage{
				Status:  connectors.HealthStatusDegraded,
				Reason:  "missing_controller",
				Message: msg,
			})
		}
	}
	for _, controller := range subsys.Controllers {
		// can't use controller.State because sometimes it's not populated
		state, err := readState(ctx, s.tr, controller.Controller)
		if err != nil {
			issues = append(issues, connectors.HealthMessage{
				Status:  connectors.HealthStatusDegraded,
				Reason:  "internal_error",
				Message: fmt.Sprintf("controller %s: failed to read state: %s", controller.Controller, err.Error()),
			})
		} else if state != "live" {
			issues = append(issues, connectors.HealthMessage{
				Status: connectors.HealthStatusDegraded,
				Reason: "controller_is_down",
				// TODO add connection name
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
		_, err := s.snip.IdNs(ctx, fmt.Sprintf("%s/%s", s.DevFolder, ns.NameSpace))
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
	if s.tr == nil {
		return fmt.Errorf("transport connector is not set")
	}
	if s.snip == nil {
		return fmt.Errorf("snippets is not set")
	}
	if s.DevFolder == "" {
		return fmt.Errorf("DevFolder is not set")
	}
	if s.Connections == nil {
		return fmt.Errorf("missing connections list")
	}
	if s.DeviceName == "" {
		return fmt.Errorf("missing DeviceName")
	}
	if s.Serial == "" {
		return fmt.Errorf("missing Serial")
	}
	err := s.Params.Validate()
	if err != nil {
		return fmt.Errorf("connect params: %w", err)
	}
	return nil
}

func readState(ctx context.Context, tr transport.Transport, dev string) (string, error) {
	state, err := tr.Cmds().Read(ctx, fmt.Sprintf("/sys/class/nvme/%s/state", dev), true)
	return string(state), err
}

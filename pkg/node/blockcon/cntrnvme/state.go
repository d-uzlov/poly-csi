package cntrnvme

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
)

type state struct {
	tr        transport.Transport `json:"-"`
	snip      *NvmeSnippets       `json:"-"`
	devFolder string              `json:"-"`

	Params      ConnectParams        `json:"params"`
	Auth        *ConnectAuth         `json:"auth"`
	ConExtra    ConnectExtra         `json:"conExtra"`
	Connections []ExpectedConnection `json:"connections"`

	Devices map[int]string `json:"devices"`
	Serial  string         `json:"serial"`
}

type ExpectedConnection struct {
	Addr       string `json:"addr"`
	Controller string `json:"controller"`
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

func (s *state) Connect(ctx context.Context) error {
	successCount := 0
	for i, v := range s.Connections {
		controllerName, err := s.snip.Connect(ctx, v.Addr, s.Params, s.Auth, &s.ConExtra)
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
	s.Devices = map[int]string{}
	for _, ns := range subsys.Namespaces {
		s.Devices[ns.NSID-1] = ns.NameSpace
	}
	if len(s.Devices) == 0 {
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
		// for i, ec := range s.Connections {
		// 	if ec.Controller == v.Controller {
		// 		s.Connections[i].Ip = getIpFromNvmeAddr(v.Address)
		// 	}
		// }
	}
	return nil
}

func (s *state) GetDevices(ctx context.Context) ([]string, error) {
	// TODO
	index := 0
	dev, ok := s.Devices[index]
	if !ok {
		return nil, fmt.Errorf("device not found")
	}
	namespaceDevicePath := s.devFolder + "/" + dev
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

func (s *state) CheckHealth(ctx context.Context) ([]blockapi.HealthMessage, error) {
	var issues []blockapi.HealthMessage
	subsys, err := s.snip.GetSubsysInfo(ctx, s.Params.TargetNqn)
	if err != nil {
		return nil, err
	}
	if subsys == nil {
		issues = append(issues, blockapi.HealthMessage{
			Status:  blockapi.HealthStatusInaccessible,
			Reason:  "NotConnected",
			Message: "subsystem is no longer connected to this host",
		})
		return issues, nil
	} else if len(subsys.Controllers) == 0 {
		issues = append(issues, blockapi.HealthMessage{
			Status:  blockapi.HealthStatusInaccessible,
			Reason:  "NotConnected",
			Message: "subsystem is not associated with any controller",
		})
		return issues, nil
	} else if len(subsys.Namespaces) == 0 {
		issues = append(issues, blockapi.HealthMessage{
			Status:  blockapi.HealthStatusInaccessible,
			Reason:  "NoNamespaces",
			Message: "subsystem does not have any namespaces",
		})
		return issues, nil
	}

	var missingControllers []string

	for _, ec := range s.Connections {
		found := false
		for _, controller := range subsys.Controllers {
			if ec.Controller == controller.Controller {
				found = true
			}
		}
		if !found {
			if ec.Controller == "" {
				missingControllers = append(missingControllers, fmt.Sprintf("missing controller for %s", ec.Addr))
			} else {
				missingControllers = append(missingControllers, fmt.Sprintf("missing controller for %s (was %s)", ec.Addr, ec.Controller))
			}
		}
	}

	var internalErrors []string
	for _, controller := range subsys.Controllers {
		// can't use controller.State because sometimes it's not populated
		state, err := readState(ctx, s.tr, controller.Controller)
		if err != nil {
			internalErrors = append(internalErrors, fmt.Sprintf("failed to read state of %s: %s", controller.Controller, err.Error()))
		} else if state != "live" {
			missingControllers = append(missingControllers, fmt.Sprintf("controller %s: state is not 'live': %s", controller.Controller, state))
		}
	}

	if len(missingControllers) > 0 {
		issues = append(issues, blockapi.HealthMessage{
			Status:  blockapi.HealthStatusDegraded,
			Reason:  "MultipathLoss",
			Message: strings.Join(missingControllers, ", "),
		})
	}
	if len(missingControllers) > 0 {
		issues = append(issues, blockapi.HealthMessage{
			Status:  blockapi.HealthStatusDegraded,
			Reason:  "InternalError",
			Message: strings.Join(internalErrors, ", "),
		})
	}

	var failedNamespaces []string
	for _, ns := range subsys.Namespaces {
		_, err := s.snip.IdNs(ctx, fmt.Sprintf("%s/%s", s.devFolder, ns.NameSpace))
		if err != nil {
			failedNamespaces = append(failedNamespaces, fmt.Sprintf("namespace %s: failed to run id-ns", ns.NameSpace))
		}
	}
	if len(missingControllers) > 0 {
		issues = append(issues, blockapi.HealthMessage{
			Status:  blockapi.HealthStatusInaccessible,
			Reason:  "NamespaceFailed",
			Message: strings.Join(failedNamespaces, ", "),
		})
	}

	return issues, nil
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
		return fmt.Errorf("transport connector is not set")
	}
	if s.snip == nil {
		return fmt.Errorf("snippets is not set")
	}
	if s.devFolder == "" {
		return fmt.Errorf("DevFolder is not set")
	}
	if s.Connections == nil {
		return fmt.Errorf("missing connections list")
	}
	if len(s.Devices) == 0 {
		return fmt.Errorf("missing devices")
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

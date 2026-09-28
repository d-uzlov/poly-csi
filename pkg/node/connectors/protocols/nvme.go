package protocols

import (
	"fmt"
)

type Nvme struct {
	// public
	TargetNqn     string `map:"nvme.targetNqn,omitempty"     json:"targetNqn,omitempty"`
	HostNqn       string `map:"nvme.hostNqn,omitempty"       json:"hostNqn,omitempty"`
	HostId        string `map:"nvme.hostId,omitempty"        json:"hostId,omitempty"`
	AuthType      string `map:"nvme.authType,omitempty"      json:"authType,omitempty"`
	ConnectPort   string `map:"nvme.connectPort,omitempty"   json:"connectPort,omitempty"`
	TransportType string `map:"nvme.transportType,omitempty" json:"transportType,omitempty"`

	// secret
	HostKey       string `secret:"+" map:"nvme.hostKey"       json:"hostKey,omitempty"`
	ControllerKey string `secret:"+" map:"nvme.controllerKey" json:"controllerKey,omitempty"`
}

const (
	NvmeAuthNqn    = "nqn"
	NvmeAuthDhChap = "dhchap-mutual"
)

func (v Nvme) Validate() error {
	if v.TargetNqn == "" {
		return fmt.Errorf("target nqn is empty")
	}
	if v.HostNqn == "" {
		return fmt.Errorf("host nqn is empty")
	}
	if v.HostId == "" {
		return fmt.Errorf("host id is empty")
	}
	switch v.AuthType {
	case NvmeAuthNqn, NvmeAuthDhChap:
		// ok
	default:
		return fmt.Errorf("unknown auth type: %s", v.AuthType)
	}
	if v.ConnectPort == "" {
		return fmt.Errorf("connect port is empty")
	}
	if v.TransportType == "" {
		return fmt.Errorf("transport type is empty")
	}
	switch v.AuthType {
	case "":
		return fmt.Errorf("missing auth type")
	case NvmeAuthNqn:
		// nothing to do here
	case NvmeAuthDhChap:
		if v.HostKey == "" {
			return fmt.Errorf("missing host key")
		}
		if v.ControllerKey == "" {
			return fmt.Errorf("missing controller key")
		}
	default:
		return fmt.Errorf("auth type: unknown value: '%s'", v.AuthType)
	}
	return nil
}

package protocols

import (
	"fmt"
)

type NvmeShare struct {
	// public
	TargetNqn     string `map:"nvme.targetNqn,omitempty"     json:"targetNqn,omitempty"     validate:"required"`
	HostNqn       string `map:"nvme.hostNqn,omitempty"       json:"hostNqn,omitempty"       validate:"required"`
	HostId        string `map:"nvme.hostId,omitempty"        json:"hostId,omitempty"        validate:"required"`
	AuthType      string `map:"nvme.authType,omitempty"      json:"authType,omitempty"      validate:"required,oneof=nqn dhchap-mutual"`
	ConnectPort   int    `map:"nvme.connectPort,omitempty"   json:"connectPort,omitempty"   validate:"required,port"`
	TransportType string `map:"nvme.transportType,omitempty" json:"transportType,omitempty" validate:"required"`

	// secret
	HostKey       string `secret:"+" map:"nvme.hostKey"       json:"hostKey,omitempty"       validate:"required_if=AuthType dhchap-mutual"`
	ControllerKey string `secret:"+" map:"nvme.controllerKey" json:"controllerKey,omitempty" validate:"required_if=AuthType dhchap-mutual"`
}

const (
	NvmeAuthNqn    = "nqn"
	NvmeAuthDhChap = "dhchap-mutual"
)

func (v NvmeShare) Validate() error {
	switch v.AuthType {
	case NvmeAuthNqn, NvmeAuthDhChap:
		// ok
	default:
		return fmt.Errorf("unknown auth type: %s", v.AuthType)
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

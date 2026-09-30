package protocols

import (
	"fmt"
)

const (
	IscsiAuthNone = "none"
	IscsiAuthChap = "chap-mutual"
)

type IscsiShare struct {
	// public
	Initiator string `map:"iscsi.initiator,omitempty"     json:"initiator,omitempty"`
	Iqn       string `map:"iscsi.nqn,omitempty"           json:"iqn,omitempty"`
	Port      string `map:"iscsi.port,omitempty"          json:"port,omitempty"`
	Transport string `map:"iscsi.transport,omitempty"     json:"transport,omitempty"`
	AuthType  string `map:"iscsi.authType,omitempty"      json:"authType,omitempty"`

	// secret
	ClientUser string `secret:"+" map:"iscsi.clientUser" json:"clientUser,omitempty"`
	ClientPass string `secret:"+" map:"iscsi.clientPass" json:"clientPass,omitempty"`
	TargetUser string `secret:"+" map:"iscsi.targetUser" json:"targetUser,omitempty"`
	TargetPass string `secret:"+" map:"iscsi.targetPass" json:"targetPass,omitempty"`
}

func (v IscsiShare) Validate() error {
	if v.Initiator == "" {
		return fmt.Errorf("missing Initiator")
	}
	if v.Iqn == "" {
		return fmt.Errorf("missing iqn")
	}
	if v.Port == "" {
		return fmt.Errorf("missing port")
	}
	if v.Transport == "" {
		return fmt.Errorf("missing transport")
	}
	switch v.AuthType {
	case "":
		return fmt.Errorf("missing auth type")
	case IscsiAuthNone:
		// nothing to do here
	case IscsiAuthChap:
		if v.ClientUser == "" {
			return fmt.Errorf("missing client user")
		}
		if v.ClientPass == "" {
			return fmt.Errorf("missing client pass")
		}
		if v.TargetUser == "" {
			return fmt.Errorf("missing target user")
		}
		if v.TargetPass == "" {
			return fmt.Errorf("missing target pass")
		}
	default:
		return fmt.Errorf("auth type: unknown value: '%s'", v.AuthType)
	}
	return nil
}

type IscsiNodeInfo struct {
	InitiatorIqn string `map:"initiatorIqn"`
}

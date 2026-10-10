package protocols

import (
	"fmt"
)

const Iscsi = Name("iscsi")

type IscsiChap string

const (
	IscsiAuthNone = IscsiChap("none")
	IscsiAuthChap = IscsiChap("chap-mutual")
)

type IscsiShare struct {
	// public
	Initiator string    `map:"iscsi.initiator,omitempty"    json:"initiator,omitempty" validate:"required"`
	Iqn       string    `map:"iscsi.nqn,omitempty"          json:"iqn,omitempty"       validate:"required"`
	Port      int       `map:"iscsi.port,omitempty"         json:"port,omitempty"      validate:"required,port"`
	Transport string    `map:"iscsi.transport,omitempty"    json:"transport,omitempty" validate:"required"`
	AuthType  IscsiChap `map:"iscsi.authType,omitempty"     json:"authType,omitempty"  validate:"required,oneof=none chap-mutual"`

	// secret
	ClientUser string `secret:"+" map:"iscsi.clientUser,omitempty" json:"clientUser,omitempty" validate:"required_if=AuthType chap-mutual"`
	ClientPass string `secret:"+" map:"iscsi.clientPass,omitempty" json:"clientPass,omitempty" validate:"required_if=AuthType chap-mutual"`
	TargetUser string `secret:"+" map:"iscsi.targetUser,omitempty" json:"targetUser,omitempty" validate:"required_if=AuthType chap-mutual"`
	TargetPass string `secret:"+" map:"iscsi.targetPass,omitempty" json:"targetPass,omitempty" validate:"required_if=AuthType chap-mutual"`
}

var _ Config = IscsiShare{}

func (v IscsiShare) Validate() error {
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

func (v IscsiShare) RegisterSerialmapsType() string {
	return string(Iscsi)
}

func (v IscsiShare) Clone() Config {
	return v
}

type IscsiNodeInfo struct {
	InitiatorIqn string `map:"initiatorIqn"`
}

var _ NodeInfo = IscsiNodeInfo{}

func (i IscsiNodeInfo) Validate() error {
	return nil
}

func (v IscsiNodeInfo) RegisterSerialmapsType() string {
	return string(Iscsi)
}

func (v IscsiNodeInfo) Clone() Config {
	return v
}

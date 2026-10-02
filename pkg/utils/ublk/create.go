package ublk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

type CreateParams struct {
	DevNum string
	Type   string
	// enable zero-copy in io_uring
	Zerocopy bool
}

func (p CreateParams) GetForegroundBinary() string {
	return "ublk." + p.Type
}

func (c CreateParams) MakeParams() []string {
	result := []string{
		"--user_recovery", "1",
	}
	if c.Zerocopy {
		result = append(result, "--zerocopy")
	}
	if c.DevNum != "" {
		result = append(result, "-n", c.DevNum)
	}
	return result
}

type ExtraParams interface {
	MakeParams() []string
}

func (s *Snippets) Create(ctx context.Context, cr CreateParams, extra ExtraParams) (DeviceInfo, error) {
	cmd := append(ublkCmd, "add", "--type", cr.Type)
	cmd = append(cmd, cr.MakeParams()...)
	cmd = append(cmd, extra.MakeParams()...)

	rr, err := s.tr.RunCommand(ctx, cmd)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("ublk add: %w", err)
	}
	if rr.ExitCode != 0 {
		return DeviceInfo{}, fmt.Errorf("ublk add: %w", transport.FormatRunError(rr))
	}

	info, err := ParseUblkOutput(string(rr.Stdout))
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("parse ublk add output: %w", err)
	}

	return info, nil
}

// NBD

type CreateNbd struct {
	Host       string
	Port       string
	ExportName string
	// enable zero-copy for NBD connection
	SendZeroCopy bool
}

func (c CreateNbd) MakeParams() []string {
	result := []string{
		"--host", c.Host,
		"--export_name", c.ExportName,
	}
	if c.Port != "" {
		result = append(result, "--port", c.Port)
	}
	if c.SendZeroCopy {
		result = append(result, "--send_zc", "1")
	}
	return result
}

type NbdTarget struct {
	DevSize    int64  `json:"dev_size"`
	ExportName string `json:"export_name"`
	Host       string `json:"host"`
	Port       string `json:"port"`
}

func (di DeviceInfo) ParseTarget[T any]() (T, error) {
	var result T
	if di.Target == "" {
		return result, fmt.Errorf("target info is empty")
	}
	err := json.Unmarshal([]byte(di.Target), &result)
	if err != nil {
		return result, fmt.Errorf("parse target info '%s': %w", di.Target, err)
	}
	return result, nil
}

// iSCSI

type CreateIscsi struct {
	InitiatorName string
	Url           string
}

func (c *CreateIscsi) MakeParams() []string {
	result := []string{
		"--initiator-name", c.InitiatorName,
		"--iscsi", c.Url,
	}
	return result
}

type IscsiTarget struct {
	DevSize       int64  `json:"dev_size"`
	InitiatorName string `json:"initiator-name"`
	Url           string `json:"url"`
}

type IscsiUrlParams struct {
	Host      string
	Port      string
	TargetIqn string
	Lun       string

	Username       string
	Password       string
	TargetUsername string
	TargetPassword string
}

func (u IscsiUrlParams) Validate() error {
	if u.Host == "" {
		return fmt.Errorf("missing Host")
	}
	if u.Port == "" {
		return fmt.Errorf("missing Port")
	}
	if u.TargetIqn == "" {
		return fmt.Errorf("missing TargetIqn")
	}
	if u.Lun == "" {
		return fmt.Errorf("missing Lun")
	}

	if u.Username != "" && u.Password == "" {
		return fmt.Errorf("has Username but not Password")
	}
	if u.Username == "" && u.Password != "" {
		return fmt.Errorf("has Password but not Username")
	}
	if u.TargetUsername != "" && u.TargetPassword == "" {
		return fmt.Errorf("has TargetUsername but not TargetPassword")
	}
	if u.TargetUsername == "" && u.TargetPassword != "" {
		return fmt.Errorf("has TargetPassword but not TargetUsername")
	}
	return nil
}

// iscsi://[<username>[%<password>]@]<host>[:<port>]/<target-iqn>/<lun>[?<argument>[&<argument>]*]
func (u IscsiUrlParams) String() string {
	result := "iscsi://"
	if u.Username != "" && u.Password != "" {
		result += u.Username + "%" + u.Password + "@"
	}
	result += u.Host
	if u.Port != "" {
		result += ":" + u.Port
	}
	result += "/" + u.TargetIqn + "/" + u.Lun
	var params []string
	if u.TargetUsername != "" && u.TargetPassword != "" {
		params = append(params, "target_user="+u.TargetUsername, "target_password="+u.TargetPassword)
	}
	if len(params) > 0 {
		result += "?" + strings.Join(params, "&")
	}
	return result
}

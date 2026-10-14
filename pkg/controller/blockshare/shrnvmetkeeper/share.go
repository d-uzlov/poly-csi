package shrnvmetkeeper

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/blockshare"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/nvmet"
)

type share struct {
	tr transport.Transport

	Nqn          string `json:"nqn"`
	TcpPort      int    `json:"tcp_port"`
	NvmePortName string `json:"nvme_port_name"`
	Transport    string `json:"transport"`
	AuthType     string `json:"auth_type"`
}

var _ blockshare.Share = &share{}

func parseShare(data json.RawMessage) (*share, error) {
	var result share
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (sh *share) Serialize() (json.RawMessage, error) {
	return json.Marshal(sh)
}

func (sh *share) Validate() error {
	if sh.tr == nil {
		return fmt.Errorf("transport is nil")
	}
	if sh.Nqn == "" {
		return fmt.Errorf("missing nqn")
	}
	if sh.Transport == "" {
		return fmt.Errorf("missing transport")
	}
	if sh.AuthType == "" {
		return fmt.Errorf("missing auth_type")
	}
	return nil
}

func (sh *share) PrepareShare(ctx context.Context) error {
	allowAnyHost := false
	if sh.AuthType == authTypeNone {
		allowAnyHost = true
	}

	cmd := append(nvmetKeeperCmd, "subsys", "create", nvmet.Subsys{
		Nqn:          sh.Nqn,
		AllowAnyHost: allowAnyHost,
	}.Json())
	err := transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return fmt.Errorf("subsys create: %w", err)
	}

	cmd = append(nvmetKeeperCmd, "subsys", "attach", sh.Nqn, sh.NvmePortName)
	err = transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return fmt.Errorf("subsys attach: %w", err)
	}

	return nil
}

func (sh *share) AttachPath(ctx context.Context, fsPath string, index int) error {
	cmd := append(nvmetKeeperCmd, "namespace", "create", nvmet.Ns{
		Nqn:        sh.Nqn,
		Nsid:       fmt.Sprintf("%d", index+1),
		DevicePath: fsPath,
		Enabled:    true,
	}.Json())
	err := transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return fmt.Errorf("namespace create: %w", err)
	}

	return nil
}

func (sh *share) AttachVolume(ctx context.Context, volume any, index int) error {
	return fmt.Errorf("unimplemented")
}

func (sh *share) DeleteShare(ctx context.Context) error {
	cmd := append(nvmetKeeperCmd, "subsys", "detach", sh.Nqn, sh.NvmePortName)
	err := transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return fmt.Errorf("subsys detach: %w", err)
	}

	cmd = append(nvmetKeeperCmd, "subsys", "delete", sh.Nqn)
	err = transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return fmt.Errorf("subsys delete: %w", err)
	}

	return nil
}

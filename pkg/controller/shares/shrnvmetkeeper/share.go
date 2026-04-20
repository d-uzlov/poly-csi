package shrnvmetkeeper

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport

	Nqn       string `json:"nqn"`
	Port      string `json:"port"`
	Transport string `json:"transport"`
	IoQueues  string `json:"io_queues"`
	AuthType  string `json:"auth_type"`
}

var _ shares.Share = &share{}

func parseShare(data string) (*share, error) {
	var result share
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (sh *share) Serialize() (string, error) {
	encoded, err := json.Marshal(sh)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (sh *share) Validate() error {
	if sh.tr == nil {
		return fmt.Errorf("transport is nil")
	}
	if sh.Nqn == "" {
		return fmt.Errorf("missing nqn")
	}
	if sh.Port == "" {
		return fmt.Errorf("missing port")
	}
	if sh.Transport == "" {
		return fmt.Errorf("missing transport")
	}
	if sh.AuthType == "" {
		return fmt.Errorf("missing auth_type")
	}
	return nil
}

func (sh *share) DeleteShare(ctx context.Context) error {
	cmd := append(nvmetKeeperCmd, "subsys", "detach", sh.Nqn, sh.Port)
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

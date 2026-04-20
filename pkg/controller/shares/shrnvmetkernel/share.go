package shrnvmetkernel

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/nvmet"
)

type share struct {
	tr transport.Transport

	Nqn       string `json:"nqn"`
	Port      string `json:"port"`
	Transport string `json:"transport"`
	IoQueues  string `json:"io_queues"`
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
	return nil
}

func (sh *share) DeleteShare(ctx context.Context) error {
	if !testReady(ctx, sh.tr) {
		return fmt.Errorf("nvmet is not ready")
	}

	snip := nvmet.NewSnippets(ctx, sh.tr, true)

	err := snip.DetachNqnFromPort(sh.Nqn, "1")
	if err != nil {
		return fmt.Errorf("detach port: %w", err)
	}
	err = snip.DeleteNamespace(sh.Nqn, "1")
	if err != nil {
		return fmt.Errorf("delete namespace: %w", err)
	}
	err = snip.DeleteSubsys(sh.Nqn)
	if err != nil {
		return fmt.Errorf("delete subsys: %w", err)
	}

	err = saveState(ctx, sh.tr)
	if err != nil {
		return fmt.Errorf("nvmet_save: %w", err)
	}

	return nil
}

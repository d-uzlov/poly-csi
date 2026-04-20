package shrtargetcli

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport
	gp globalParams

	Iqn           string `json:"iqn"`
	Port          string `json:"port"`
	Transport     string `json:"transport"`
	BackstoreName string `json:"backstore_name"`
	AuthType      string `json:"auth_type"`
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
	if sh.gp.iqnBase == "" {
		return fmt.Errorf("iqn base is not set")
	}
	if sh.Iqn == "" {
		return fmt.Errorf("missing iqn")
	}
	if sh.BackstoreName == "" {
		return fmt.Errorf("missing backstoreName")
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
		return fmt.Errorf("targetcli is not ready")
	}

	snip := newTargetcliSnippets(sh.tr)

	err := snip.deleteTarget(ctx, sh.Iqn)
	if err != nil {
		return fmt.Errorf("delete target: %w", err)
	}
	err = snip.deleteBackstore(ctx, sh.BackstoreName)
	if err != nil {
		return fmt.Errorf("delete backstore: %w", err)
	}
	err = snip.save(ctx)
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

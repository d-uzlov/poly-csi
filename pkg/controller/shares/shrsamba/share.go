package shrsamba

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

	ConfigFile string `json:"config_file"`
	ShareName  string `json:"share_name"`
	UserName   string `json:"user_name"`
	UserPass   string `json:"user_pass"`
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
	if sh.gp.sambaConfigDir == "" {
		return fmt.Errorf("missing samba config dir")
	}
	if sh.ConfigFile == "" {
		return fmt.Errorf("missing config_file")
	}
	if sh.ShareName == "" {
		return fmt.Errorf("missing share_name")
	}
	if sh.UserName == "" {
		return fmt.Errorf("missing user_name")
	}
	if sh.UserPass == "" {
		return fmt.Errorf("missing user_pass")
	}
	return nil
}

func (sh *share) DeleteShare(ctx context.Context) error {
	snip := newSambaSnippets(sh.tr, sh.gp)

	err := snip.removeShare(ctx, sh.ConfigFile)
	if err != nil {
		return fmt.Errorf("remove share config: %w", err)
	}

	err = snip.deleteUser(ctx, sh.UserName)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}

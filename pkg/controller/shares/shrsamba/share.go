package shrsamba

import (
	"context"
	"encoding/json"
	"fmt"
	"uuid"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport
	gp globalParams

	ShareName string `json:"share_name"`
	UserName  string `json:"user_name"`
	UserPass  string `json:"user_pass"`
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
	if sh.ShareName == "" {
		return fmt.Errorf("missing ShareName")
	}
	// if sh.UserName == "" {
	// 	return fmt.Errorf("missing UserName")
	// }
	// if sh.UserPass == "" {
	// 	return fmt.Errorf("missing UserPass")
	// }
	return nil
}

func (sh *share) SharePath(ctx context.Context, fsPath string, index int) error {
	snip := newSambaSnippets(sh.tr, sh.gp)

	sh.UserName = fmt.Sprintf("smb-for-%s", sh.ShareName)
	sh.UserPass = uuid.NewV4().String()

	err := snip.configureUser(ctx, sh.UserName, sh.UserPass)
	if err != nil {
		return fmt.Errorf("configure user: %w", err)
	}

	err = snip.configureShare(ctx, sh.ShareName, ShareParameters{
		ShareName: sh.ShareName,
		Path:      fsPath,
		User:      sh.UserName,
	})
	if err != nil {
		return fmt.Errorf("configureShare: %w", err)
	}

	return nil
}

func (sh *share) ShareVolume(ctx context.Context, volume any, index int) error {
	return fmt.Errorf("unimplemented")
}

func (sh *share) DeleteShare(ctx context.Context) error {
	snip := newSambaSnippets(sh.tr, sh.gp)

	err := snip.removeShare(ctx, sh.ShareName)
	if err != nil {
		return fmt.Errorf("remove share config: %w", err)
	}

	err = snip.deleteUser(ctx, sh.UserName)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}

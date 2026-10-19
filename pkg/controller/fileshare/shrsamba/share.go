package shrsamba

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/fileshare"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport
	gp globalParams

	ShareName string `json:"share_name"`
	UserName  string `json:"user_name"`
	UserPass  string `json:"user_pass"`
	Secret    []byte `json:"secret"`
}

var _ fileshare.Share = &share{}

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

func (sh *share) PrepareShare(ctx context.Context) error {
	sh.Secret = make([]byte, 32)
	_, err := rand.Read(sh.Secret)
	if err != nil {
		return err
	}

	h := hmac.New(sha256.New, sh.Secret)
	h.Write([]byte(sh.ShareName))
	passBytes := h.Sum(nil)

	sh.UserName = fmt.Sprintf("smb-for-%s", sh.ShareName)

	sh.UserPass = base64.RawStdEncoding.EncodeToString(passBytes)

	return nil
}

func (sh *share) AttachPath(ctx context.Context, fsPath string) error {
	snip := newSambaSnippets(sh.tr, sh.gp)

	err := snip.configureUser(ctx, sh.UserName, sh.UserPass)
	if err != nil {
		return fmt.Errorf("configure user: %w", err)
	}

	err = sh.tr.SudoCmds().Chown(ctx, fsPath, sh.UserName, sh.UserName, true)
	if err != nil {
		return fmt.Errorf("chown to share user: %w", err)
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

func (sh *share) AttachVolume(ctx context.Context, volume any) error {
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

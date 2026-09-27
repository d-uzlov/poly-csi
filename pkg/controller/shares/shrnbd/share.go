package shrnbd

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport `json:"-"`
	gp globalParams        `json:"-"`

	ConnectPort string `json:"connect_port"`
	ExportName  string `json:"export_name"`
	DevicePath  string `json:"user_name"`
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
	if sh.ConnectPort == "" {
		return fmt.Errorf("missing ConnectPort")
	}
	if sh.ExportName == "" {
		return fmt.Errorf("missing ExportName")
	}
	// if sh.DevicePath == "" {
	// 	return fmt.Errorf("missing DevicePath")
	// }
	return nil
}

func (sh *share) PrepareShare(ctx context.Context) error {
	return nil
}

func (sh *share) AttachPath(ctx context.Context, fsPath string, index int) error {
	snip := newSnippets(sh.tr, sh.gp)

	err := snip.configureShare(ctx, sh.ExportName, ShareConfig{
		ExportName: sh.ExportName,
		DevicePath: fsPath,
	})
	if err != nil {
		return fmt.Errorf("configureShare: %w", err)
	}

	return nil
}

func (sh *share) AttachVolume(ctx context.Context, volume any, index int) error {
	return fmt.Errorf("unimplemented")
}

func (sh *share) DeleteShare(ctx context.Context) error {
	snip := newSnippets(sh.tr, sh.gp)

	err := snip.removeShare(ctx, sh.ExportName)
	if err != nil {
		return fmt.Errorf("remove share config: %w", err)
	}

	return nil
}

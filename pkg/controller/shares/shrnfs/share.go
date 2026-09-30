package shrnfs

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/protocols"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport

	Name   string `json:"name"`
	FsPath string `json:"fs_path"`
	Params Params `json:"params"`
}

var _ shares.Share = &share{}

func (sh *share) Serialize() (string, error) {
	encoded, err := json.Marshal(sh)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func parseShare(shareData string) (*share, error) {
	var result share
	err := json.Unmarshal([]byte(shareData), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (sh *share) Validate() error {
	if sh.tr == nil {
		return fmt.Errorf("transport is nil")
	}
	if sh.Name == "" {
		return fmt.Errorf("missing name")
	}
	if sh.FsPath == "" {
		return fmt.Errorf("missing FsPath")
	}
	return nil
}

func (sh *share) PrepareShare(ctx context.Context) error {
	return nil
}

func (sh *share) AttachPath(ctx context.Context, fsPath string, index int) error {
	snip := newNfsSnippets(sh.tr)

	sh.FsPath = fsPath

	return snip.configureNfsShare(ctx, sh.Name, nfsShareConfig{
		path:        fsPath,
		globalFlags: sh.Params.shareOptions,
		clients: []nfsClientOptions{
			{id: "*"},
		},
	})
}

func (sh *share) AttachVolume(ctx context.Context, volume any, index int) error {
	return fmt.Errorf("unimplemented")
}

func (sh *share) DeleteShare(ctx context.Context) error {
	snip := newNfsSnippets(sh.tr)

	return snip.removeNfsShare(ctx, sh.Name)
}

func (sh *share) Publish(ctx context.Context, req shares.PublishRequest) (*shares.PublishContext, error) {
	return &shares.PublishContext{
		Context: protocols.NfsShare{
			Path: sh.FsPath,
		},
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized string) error {
	return nil
}

func (sh *share) UnpublishAll(ctx context.Context) error {
	return nil
}

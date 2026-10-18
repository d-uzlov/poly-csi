package shrnfs

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/fsapi"
	"meoe.io/poly-csi/pkg/controller/fileshare"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport

	Name   string `json:"name"`
	FsPath string `json:"fs_path"`
	Params Params `json:"params"`
}

var _ fileshare.Share = &share{}

func (sh *share) Serialize() (json.RawMessage, error) {
	return json.Marshal(sh)
}

func parseShare(shareData json.RawMessage) (*share, error) {
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

func (sh *share) AttachPath(ctx context.Context, fsPath string) error {
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

func (sh *share) AttachVolume(ctx context.Context, volume any) error {
	return fmt.Errorf("unimplemented")
}

func (sh *share) DeleteShare(ctx context.Context) error {
	snip := newNfsSnippets(sh.tr)

	return snip.removeNfsShare(ctx, sh.Name)
}

func (sh *share) Publish(ctx context.Context, req fileshare.PublishRequest) (*fileshare.PublishContext, error) {
	if len(req.AddressList) != 1 {
		return nil, fmt.Errorf("nfs needs exactly one server address")
	}
	return &fileshare.PublishContext{
		Protocol: fsapi.NfsShare{
			Path:          sh.FsPath,
			ServerAddress: req.AddressList[0],
		},
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized json.RawMessage) error {
	return nil
}

func (sh *share) UnpublishAll(ctx context.Context) error {
	return nil
}

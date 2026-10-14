package shrtnnfs

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

	FsPath string `json:"folder"`
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
	if sh.FsPath == "" {
		return fmt.Errorf("missing folder")
	}
	return nil
}

func (sh *share) PrepareShare(ctx context.Context) error {
	return nil
}

func (sh *share) AttachPath(ctx context.Context, fsPath string) error {
	snip := newNfsSnippets(sh.tr)

	err := snip.configureNfsShare(ctx, fsPath)
	if err != nil {
		return err
	}

	sh.FsPath = fsPath

	return nil
}

func (sh *share) AttachVolume(ctx context.Context, volume any) error {
	return fmt.Errorf("unimplemented")
}

func (sh *share) Publish(ctx context.Context, req fileshare.PublishRequest) (*fileshare.PublishContext, error) {
	return &fileshare.PublishContext{
		Protocol: fsapi.NfsShare{
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

func (sh *share) DeleteShare(ctx context.Context) error {
	snip := newNfsSnippets(sh.tr)

	return snip.removeNfsShare(ctx, sh.FsPath)
}

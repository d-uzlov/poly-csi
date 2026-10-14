package shrzfssharenfs

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/fsapi"
	"meoe.io/poly-csi/pkg/controller/fileshare"
	"meoe.io/poly-csi/pkg/controller/filevol/strzfsdataset"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type share struct {
	tr transport.Transport

	ShareOptions string `json:"shareOptions"`
	Dataset      string `json:"dataset"`
	Folder       string `json:"folder"`
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
	return nil
}

func (sh *share) PrepareShare(ctx context.Context) error {
	return nil
}

func (sh *share) AttachPath(ctx context.Context, fsPath string) error {
	return fmt.Errorf("unimplemented")
}

func (sh *share) AttachVolume(ctx context.Context, volume any) error {

	dataset, ok := volume.(*strzfsdataset.Volume)
	if !ok {
		return fmt.Errorf("storage volume is not a zfs dataset: %T", volume)
	}

	sh.Dataset = dataset.Path

	state, err := dataset.ReadState(ctx)
	if err != nil {
		return fmt.Errorf("read dataset state: %w", err)
	}
	sh.Folder = state.FsPath

	snip := zfs.NewSnippets(sh.tr)

	err = snip.SetShareNfs(ctx, dataset.Path, sh.ShareOptions)
	if err != nil {
		return err
	}

	return nil
}

func (sh *share) DeleteShare(ctx context.Context) error {
	snip := zfs.NewSnippets(sh.tr)

	return snip.SetShareNfs(ctx, sh.Dataset, "off")
}

func (sh *share) Publish(ctx context.Context, req fileshare.PublishRequest) (*fileshare.PublishContext, error) {
	return &fileshare.PublishContext{
		Protocol: fsapi.NfsShare{
			Path: sh.Folder,
		},
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized string) error {
	return nil
}

func (sh *share) UnpublishAll(ctx context.Context) error {
	return nil
}

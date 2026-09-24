package shrzfssharenfs

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage/strzfsdataset"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrnfs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/zfs"
)

type share struct {
	tr transport.Transport

	ShareOptions string `json:"shareOptions"`
	Dataset      string `json:"dataset"`
	Folder       string `json:"folder"`
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
	if sh.Dataset == "" {
		return fmt.Errorf("missing dataset")
	}
	if sh.Folder == "" {
		return fmt.Errorf("missing folder")
	}
	return nil
}

func (sh *share) SharePath(ctx context.Context, fsPath string) error {
	return fmt.Errorf("unimplemented")
}

func (sh *share) ShareVolume(ctx context.Context, volume any) error {

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

func (sh *share) Publish(ctx context.Context, mi connectors.ConnectorNodeInfo) (*shares.PublishContext, error) {
	return &shares.PublishContext{
		Public: cntrnfs.PublicContext{
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

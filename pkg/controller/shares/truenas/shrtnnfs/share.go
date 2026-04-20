package shrtnnfs

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrnfs"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport

	Folder string `json:"folder"`
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
	if sh.Folder == "" {
		return fmt.Errorf("missing folder")
	}
	return nil
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

func (sh *share) DeleteShare(ctx context.Context) error {
	snip := newNfsSnippets(sh.tr)

	return snip.removeNfsShare(ctx, sh.Folder)
}

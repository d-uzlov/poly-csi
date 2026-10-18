package shrsamba

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/fsapi"
	"meoe.io/poly-csi/pkg/controller/fileshare"
)

func (sh *share) Publish(ctx context.Context, req fileshare.PublishRequest) (*fileshare.PublishContext, error) {
	if len(req.AddressList) != 1 {
		return nil, fmt.Errorf("samba needs exactly 1 server address")
	}
	return &fileshare.PublishContext{
		Protocol: fsapi.SmbShare{
			ShareName:    sh.ShareName,
			UserName:     sh.UserName,
			UserPassword: sh.UserPass,
			ServerAddress: req.AddressList[0],
		},
		LocalData: nil,
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized json.RawMessage) error {
	return nil
}

func (sh *share) UnpublishAll(ctx context.Context) error {
	return nil
}

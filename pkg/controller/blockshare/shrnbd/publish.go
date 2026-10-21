package shrnbd

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/controller/blockshare"
)

func (sh *share) Publish(ctx context.Context, req blockshare.PublishRequest) (*blockshare.PublishContext, error) {
	if len(req.AddressList) != 1 {
		return nil, fmt.Errorf("nbd needs exactly one server address")
	}
	return &blockshare.PublishContext{
		Protocol: blockspec.NbdShare{
			ExportName:    sh.ExportName,
			Port:          sh.ConnectPort,
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

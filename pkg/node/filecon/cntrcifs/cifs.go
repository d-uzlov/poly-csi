package cntrcifs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/api/protocols"
	"meoe.io/poly-csi/pkg/node/connectors"
)

type connector struct {
	connectors.NoInfo

	conf connectors.CommonConfig
}

func New(conf connectors.CommonConfig) (connectors.Manager, error) {
	return &connector{
		conf: conf,
	}, nil
}

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		Protocol:       protocols.Smb,
		HasNodeInfo:    false,
		ConnectType:    connectors.ConnectTypePublishFolder,
		ManagedUnmount: true,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req connectors.ConnectRequest) (connectors.Handle, error) {
	if len(req.AddressOptions) != 1 {
		return nil, fmt.Errorf("smb connector does not support more than one address")
	}

	proto, ok := req.Proto.(protocols.SmbShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch")
	}

	smbPath := fmt.Sprintf("//%s/%s", req.AddressOptions[0].Address, proto.ShareName)

	return &state{
		tr:        c.conf.Tr,
		SharePath: smbPath,
		Username:  proto.UserName,
		Password:  proto.UserPassword,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData string) (connectors.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.conf.Tr

	return res, nil
}

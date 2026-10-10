package cntrnfs

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
		Protocol:       protocols.Nfs,
		HasNodeInfo:    false,
		ConnectType:    connectors.ConnectTypeFolder,
		ManagedUnmount: true,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req connectors.ConnectRequest) (connectors.Handle, error) {
	if len(req.AddressOptions) != 1 {
		return nil, fmt.Errorf("nfs connector does not support more than one address")
	}

	proto, ok := req.Proto.(protocols.NfsShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch")
	}

	nfsPath := fmt.Sprintf("%s:%s", req.AddressOptions[0].Address, proto.Path)

	return &state{
		tr:      c.conf.Tr,
		NfsPath: nfsPath,
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

package cntrcifs

import (
	"context"
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/connectors"
)

type connector struct {
	conf connectors.CommonConfig
}

func New() connectors.Connector {
	return &connector{}
}

func (c *connector) Configure(ctx context.Context, conf connectors.CommonConfig, cfg yaml.Node) error {
	c.conf = conf
	return nil
}

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		ConnectType:    connectors.ConnectTypePublishFolder,
		ManagedUnmount: true,
	}, nil
}

func (c *connector) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	return &connectors.ConnectorNodeInfo{}, nil
}

func (c *connector) GetHandle(ctx context.Context, req connectors.ConnectRequest) (connectors.ConnectionHandle, error) {
	if len(req.AddressOptions) != 1 {
		return nil, fmt.Errorf("smb connector does not support more than one address")
	}
	pc, sc, err := ParsePublishContext(req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	err = validateContext(pc, sc)
	if err != nil {
		return nil, fmt.Errorf("validateContext: %w", err)
	}

	smbPath := fmt.Sprintf("//%s/%s", req.AddressOptions[0].Address, pc.ShareName)

	return &state{
		tr:        c.conf.Tr,
		SharePath: smbPath,
		Username:  pc.UserName,
		Password:  sc.UserPassword,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData string) (connectors.ConnectionHandle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.conf.Tr

	return res, nil
}

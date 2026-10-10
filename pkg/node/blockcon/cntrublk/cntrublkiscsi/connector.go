package cntrublkiscsi

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/api/protocols"
	"meoe.io/poly-csi/pkg/node/blockcon/cntrublk"
	"meoe.io/poly-csi/pkg/node/connectors"
)

type connector struct {
	connectors.NoInfo

	conf connectors.CommonConfig
	sv   cntrublk.Supervisor
}

func New(sv cntrublk.Supervisor, conf connectors.CommonConfig) connectors.Manager {
	return &connector{
		sv:   sv,
		conf: conf,
	}
}

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		Protocol:    protocols.Iscsi,
		HasNodeInfo: false,
		ConnectType: connectors.ConnectTypeBlock,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req connectors.ConnectRequest) (connectors.Handle, error) {
	if len(req.AddressOptions) != 1 {
		return nil, fmt.Errorf("ublk iscsi connector does not support more than one address")
	}

	proto, ok := req.Proto.(protocols.IscsiShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch")
	}

	return &state{
		tr:        c.conf.Tr,
		sv:        c.sv,
		devFolder: c.conf.DevFolder,
		Uuid:      req.Uuid,
		Address:   req.AddressOptions[0].Address,
		Pc:        proto,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData string) (connectors.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.conf.Tr
	res.sv = c.sv
	res.devFolder = c.conf.DevFolder

	return res, nil
}

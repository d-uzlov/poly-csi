package cntrublkiscsi

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/node/blockcon"
	"meoe.io/poly-csi/pkg/node/blockcon/cntrublk"
)

type connector struct {
	blockcon.NoInfo

	conf blockcon.CommonConfig
	sv   cntrublk.Supervisor
}

func New(sv cntrublk.Supervisor, conf blockcon.CommonConfig) blockcon.Manager {
	return &connector{
		sv:   sv,
		conf: conf,
	}
}

func (c *connector) Capabilities(ctx context.Context) (blockcon.Capabilities, error) {
	return blockcon.Capabilities{
		Protocol:    blockapi.Iscsi,
		HasNodeInfo: false,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req blockcon.ConnectRequest) (blockcon.Handle, error) {
	proto, ok := req.Proto.(blockapi.IscsiShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch")
	}

	if len(proto.ServerAddress) != 1 {
		return nil, fmt.Errorf("ublk iscsi connector does not support more than one address")
	}

	return &state{
		tr:        c.conf.Tr,
		sv:        c.sv,
		devFolder: c.conf.DevFolder,
		Uuid:      req.Uuid,
		Address:   proto.ServerAddress[0],
		Pc:        proto,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData string) (blockcon.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.conf.Tr
	res.sv = c.sv
	res.devFolder = c.conf.DevFolder

	return res, nil
}

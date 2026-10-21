package cntrublknbd

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/node/blockcon/cntrublk"
)

type connector struct {
	blockapi.NoInfo

	conf blockapi.CommonConfig
	sv   cntrublk.Supervisor
}

func New(sv cntrublk.Supervisor, conf blockapi.CommonConfig) blockapi.Manager {
	return &connector{
		sv:   sv,
		conf: conf,
	}
}

func (c *connector) Capabilities(ctx context.Context) (blockapi.Capabilities, error) {
	return blockapi.Capabilities{
		Protocol:    blockspec.Nbd,
		HasNodeInfo: false,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req blockapi.ConnectRequest) (blockapi.Handle, error) {
	proto, ok := req.Proto.(blockspec.NbdShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch")
	}

	return &state{
		tr:        c.conf.Tr,
		sv:        c.sv,
		devFolder: c.conf.DevFolder,
		Uuid:      req.Uuid,
		Address:   proto.ServerAddress,
		Pc:        proto,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData string) (blockapi.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.conf.Tr
	res.sv = c.sv
	res.devFolder = c.conf.DevFolder

	return res, nil
}

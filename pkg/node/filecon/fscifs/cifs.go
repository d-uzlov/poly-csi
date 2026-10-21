package fscifs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/api/fsspec"
	"meoe.io/poly-csi/pkg/node/filecon"
)

type connector struct {
	conf filecon.CommonConfig
}

func New(conf filecon.CommonConfig) (filecon.Manager, error) {
	return &connector{
		conf: conf,
	}, nil
}

func (c *connector) Capabilities(ctx context.Context) (filecon.Capabilities, error) {
	return filecon.Capabilities{
		FsType:      fsspec.Smb,
		HasNodeInfo: false,
		ConnectType: filecon.ConnectTypePublishFolder,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req filecon.ConnectRequest) (filecon.Handle, error) {
	proto, ok := req.Proto.(fsspec.SmbShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch")
	}

	smbPath := fmt.Sprintf("//%s/%s", proto.ServerAddress, proto.ShareName)

	return &state{
		tr:        c.conf.Tr,
		SharePath: smbPath,
		Username:  proto.UserName,
		Password:  proto.UserPassword,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData string) (filecon.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.conf.Tr

	return res, nil
}

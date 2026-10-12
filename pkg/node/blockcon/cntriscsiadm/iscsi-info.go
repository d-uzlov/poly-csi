package cntriscsiadm

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/transport"
)

func (c *connector) GetInfo(ctx context.Context) (blockapi.NodeInfo, error) {
	iqn, err := c.parseIqn(c.commonConfig.Tr, ctx)
	if err != nil {
		return nil, err
	}

	return &blockapi.IscsiNodeInfo{
		InitiatorIqn: iqn,
	}, nil
}

func (c *connector) parseIqn(tr transport.Transport, ctx context.Context) (string, error) {
	initiatorInfo, err := tr.SudoCmds().Read(ctx, c.config.InitiatorNamePath, false)
	if err != nil {
		return "", fmt.Errorf("read initiatorname.iscsi: %w", err)
	}
	for v := range strings.Lines(string(initiatorInfo)) {
		v = strings.TrimSpace(v)
		if v == "" || v[0] == '#' {
			continue
		}
		kv := strings.SplitN(v, "=", 2)
		if len(kv) != 2 {
			return "", fmt.Errorf("could not parse initiatorname.iscsi: line %s", v)
		}
		if kv[0] == "InitiatorName" {
			return kv[1], nil
		}
	}
	return "", fmt.Errorf("could not parse initiatorname.iscsi: line with InitiatorName not found")
}

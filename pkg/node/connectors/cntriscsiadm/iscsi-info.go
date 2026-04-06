package cntriscsiadm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type IscsiPublicInfo struct {
	HostIqn string `json:"host_iqn"`
}

func (m *connector) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	iqn, err := m.parseIqn(m.tr, ctx)
	if err != nil {
		return nil, err
	}

	public, err := json.Marshal(IscsiPublicInfo{
		HostIqn: iqn,
	})
	if err != nil {
		return nil, err
	}

	return &connectors.ConnectorNodeInfo{
		Public: string(public),
	}, nil
}

func ParseIscsiInfo(src connectors.ConnectorNodeInfo) (IscsiPublicInfo, error) {
	var result IscsiPublicInfo
	err := json.Unmarshal([]byte(src.Public), &result)
	if err != nil {
		return IscsiPublicInfo{}, fmt.Errorf("parsing '%v': %w", src.Public, err)
	}
	return result, nil
}

func (m *connector) parseIqn(tr transport.Transport, ctx context.Context) (string, error) {
	initiatorInfo, err := tr.SudoCmds().Read(ctx, m.config.InitiatorNamePath, false)
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

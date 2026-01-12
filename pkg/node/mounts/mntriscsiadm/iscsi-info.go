package mntriscsiadm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/transport"
)

type IscsiPublicInfo struct {
	HostIqn string `json:"host_iqn"`
}

func (m *iscsiadmMounter) GetInfo(ctx context.Context) (*mounts.MounterNodeInfo, error) {
	iqn, err := parseIqn(ctx)
	if err != nil {
		return nil, err
	}

	public, err := json.Marshal(IscsiPublicInfo{
		HostIqn: iqn,
	})
	if err != nil {
		return nil, err
	}

	return &mounts.MounterNodeInfo{
		Id:     m.id,
		Public: string(public),
		Secret: "",
	}, nil
}

func ParseIscsiInfo(src mounts.MounterNodeInfo) (IscsiPublicInfo, error) {
	var result IscsiPublicInfo
	err := json.Unmarshal([]byte(src.Public), &result)
	if err != nil {
		return IscsiPublicInfo{}, err
	}
	return result, nil
}

func parseIqn(ctx context.Context) (string, error) {
	tr := transport.FromContext(ctx)
	initiatorInfo, err := tr.SudoCmds().Read(ctx, "/etc/iscsi/initiatorname.iscsi", false)
	if err != nil {
		return "", fmt.Errorf("cat /etc/iscsi/initiatorname.iscsi: %w", err)
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

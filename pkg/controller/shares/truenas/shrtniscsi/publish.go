package shrtniscsi

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntriscsiadm"
	"meoe.io/poly-csi/pkg/node/connectors/protocols"
)

type publishData struct {
	HostIqn string `json:"host_iqn"`
}

func (d publishData) Serialize() (string, error) {
	err := d.validate()
	if err != nil {
		return "", err
	}

	encoded, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (d publishData) validate() error {
	if d.HostIqn == "" {
		return fmt.Errorf("missing host_iqn")
	}
	return nil
}

func parsePublishData(serialized string) (publishData, error) {
	var result publishData
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return result, err
	}
	err = result.validate()
	if err != nil {
		return result, err
	}
	return result, nil
}

func (sh *share) Publish(ctx context.Context, mi connectors.ConnectorNodeInfo) (*shares.PublishContext, error) {
	iscsiInfo, err := cntriscsiadm.ParseIscsiInfo(mi)
	if err != nil {
		return nil, fmt.Errorf("parse connector info as iscsi: %w", err)
	}

	snip := newSnippets(sh.tr)

	err = snip.attachGroup(ctx, sh.Iqn, iscsiInfo.HostIqn)
	if err != nil {
		return nil, fmt.Errorf("attach iscsi group: %w", err)
	}

	pd := publishData{
		HostIqn: iscsiInfo.HostIqn,
	}

	return &shares.PublishContext{
		Context: protocols.IscsiShare{
			Iqn:       sh.Iqn,
			Port:      "3260",
			Transport: "tcp",
			AuthType:  protocols.IscsiAuthNone,
		},
		LocalData: pd,
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized string) error {
	pd, err := parsePublishData(serialized)
	if err != nil {
		return fmt.Errorf("parse publish data: %w", err)
	}

	snip := newSnippets(sh.tr)

	err = snip.detachGroup(ctx, sh.Iqn, pd.HostIqn)
	if err != nil {
		return err
	}
	return nil
}

func (sh *share) UnpublishAll(ctx context.Context) error {
	return fmt.Errorf("unimplemented")
}

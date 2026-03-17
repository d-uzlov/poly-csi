package shrtargetcli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thanhpk/randstr"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntriscsiadm"
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

func (sh *share) Publish(ctx context.Context, shareData string, mi connectors.ConnectorNodeInfo) (*shares.PublishContext, error) {
	if !sh.cmds.ready(ctx, sh.tr) {
		return nil, fmt.Errorf("targetcli is not ready")
	}

	c, err := parseShareData(shareData)
	if err != nil {
		return nil, fmt.Errorf("parse share data: %w", err)
	}

	iscsiInfo, err := cntriscsiadm.ParseIscsiInfo(mi)
	if err != nil {
		return nil, fmt.Errorf("parse connector info as iscsi: %w", err)
	}

	chap := cntriscsiadm.ChapNone
	if c.AuthType == authTypeChapProvision {
		chap = cntriscsiadm.ChapFromPublishSecret
	}
	pc := map[string]string{}
	err = cntriscsiadm.InjectPublishContext(pc, cntriscsiadm.IscsiPublishContext{
		Iqn:       c.Iqn,
		Port:      c.Port,
		Transport: c.Transport,
		Chap:      chap,
	})
	if err != nil {
		return nil, fmt.Errorf("create publish context: %w", err)
	}
	pd := publishData{
		HostIqn: iscsiInfo.HostIqn,
	}

	if c.AuthType == authTypeNone {
		return &shares.PublishContext{
			Public: pc,
			Data:   pd,
		}, nil
	}

	snip := newTargetcliSnippets(sh.tr, sh.cmds.targetcliCmd)

	err = snip.createClient(ctx, c.Iqn, iscsiInfo.HostIqn)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	if c.AuthType == authTypeIqn {
		err = snip.save(ctx)
		if err != nil {
			return nil, fmt.Errorf("save config: %w", err)
		}

		return &shares.PublishContext{
			Public: pc,
			Data:   pd,
		}, nil
	}

	secret := map[string]string{}

	acl, err := snip.readClientAcl(ctx, c.Iqn, iscsiInfo.HostIqn)
	if err != nil {
		return nil, fmt.Errorf("read client auth: %w", err)
	}
	if acl == nil {
		logs.FromCtx(ctx).MicroOp("acl is not found, generating a new one", "share-iqn", c.Iqn, "host-iqn", iscsiInfo.HostIqn)
		acl = &chapAcl{
			client:     cntriscsiadm.Chap{User: randstr.Base62(16), Pass: randstr.Base62(32)},
			controller: cntriscsiadm.Chap{User: randstr.Base62(16), Pass: randstr.Base62(32)},
		}

		err = snip.setupClientAcl(ctx, c.Iqn, iscsiInfo.HostIqn, *acl)
		if err != nil {
			return nil, fmt.Errorf("setup client acl: %w", err)
		}
	}

	acl.client.Inject(secret, "chap.client")
	acl.controller.Inject(secret, "chap.target")

	err = snip.save(ctx)
	if err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	return &shares.PublishContext{
		Public: pc,
		Secret: secret,
		Data:   pd,
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, shareData string, serialized string) error {
	if !sh.cmds.ready(ctx, sh.tr) {
		return fmt.Errorf("targetcli is not ready")
	}

	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}
	pd, err := parsePublishData(serialized)
	if err != nil {
		return fmt.Errorf("parse publish data: %w", err)
	}

	snip := newTargetcliSnippets(sh.tr, sh.cmds.targetcliCmd)

	err = snip.deleteClientAcl(ctx, c.Iqn, pd.HostIqn)
	if err != nil {
		return fmt.Errorf("delete client acl: %w", err)
	}

	err = snip.save(ctx)
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

func (sh *share) UnpublishAll(ctx context.Context, shareData string) error {
	return fmt.Errorf("UnpublishAll is not implemented")
}

package shrtargetcli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thanhpk/randstr"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mntriscsiadm"
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

func (sh *share) Publish(ctx context.Context, shareData string, mi mounts.MounterNodeInfo) (*shares.PublishContext, error) {
	if !sh.cmds.ready(ctx) {
		return nil, fmt.Errorf("targetcli is not ready")
	}

	c, err := parseShareData(shareData)
	if err != nil {
		return nil, fmt.Errorf("parse share data: %w", err)
	}

	iscsiInfo, err := mntriscsiadm.ParseIscsiInfo(mi)
	if err != nil {
		return nil, fmt.Errorf("parse mounter info as iscsi: %w", err)
	}

	chap := mntriscsiadm.ChapNone
	if c.AuthType == authTypeChapProvision {
		chap = mntriscsiadm.ChapFromPublishSecret
	}
	pc := map[string]string{}
	err = mntriscsiadm.InjectPublishContext(pc, mntriscsiadm.IscsiPublishContext{
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

	snip := newTargetcliSnippets(ctx, sh.cmds.targetcliCmd)

	err = snip.createClient(c.Iqn, iscsiInfo.HostIqn)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	if c.AuthType == authTypeIqn {
		err = snip.save()
		if err != nil {
			return nil, fmt.Errorf("save config: %w", err)
		}

		return &shares.PublishContext{
			Public: pc,
			Data:   pd,
		}, nil
	}

	secret := map[string]string{}

	acl, err := snip.readClientAcl(c.Iqn, iscsiInfo.HostIqn)
	if err != nil {
		return nil, fmt.Errorf("read client auth: %w", err)
	}
	if acl == nil {
		acl = &chapAcl{
			client:     mntriscsiadm.Chap{User: randstr.Base62(16), Pass: randstr.Base62(32)},
			controller: mntriscsiadm.Chap{User: randstr.Base62(16), Pass: randstr.Base62(32)},
		}

		err = snip.setupClientAcl(c.Iqn, iscsiInfo.HostIqn, *acl)
		if err != nil {
			return nil, fmt.Errorf("setup client acl: %w", err)
		}
	}

	acl.client.Inject(secret, "chap.client")
	acl.controller.Inject(secret, "chap.target")

	err = snip.save()
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
	if !sh.cmds.ready(ctx) {
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

	snip := newTargetcliSnippets(ctx, sh.cmds.targetcliCmd)

	err = snip.deleteClientAcl(c.Iqn, pd.HostIqn)
	if err != nil {
		return fmt.Errorf("delete client acl: %w", err)
	}

	err = snip.save()
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

func (sh *share) UnpublishAll(ctx context.Context, shareData string) error {
	return fmt.Errorf("UnpublishAll is not implemented")
}

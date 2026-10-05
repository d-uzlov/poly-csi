package shrtargetcli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thanhpk/randstr"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/protocols"
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

func (sh *share) Publish(ctx context.Context, req shares.PublishRequest) (*shares.PublishContext, error) {
	if !testReady(ctx, sh.tr) {
		return nil, fmt.Errorf("targetcli is not ready")
	}

	var iscsiInfo protocols.IscsiNodeInfo
	if req.ConInfo != nil {
		ok := false
		iscsiInfo, ok = req.ConInfo.(protocols.IscsiNodeInfo)
		if !ok {
			return nil, fmt.Errorf("node info is not iscsi")
		}
	}

	if iscsiInfo.InitiatorIqn == "" {
		iscsiInfo.InitiatorIqn = fmt.Sprintf("%s:host:%s", sh.gp.iqnBase, req.NodeName)
	}

	chap := protocols.IscsiAuthNone
	if sh.AuthType == authTypeChapProvision {
		chap = protocols.IscsiAuthChap
	}
	pc := protocols.IscsiShare{
		Initiator: iscsiInfo.InitiatorIqn,
		Iqn:       sh.Iqn,
		Port:      sh.Port,
		Transport: sh.Transport,
		AuthType:  chap,
	}
	pd := publishData{
		HostIqn: iscsiInfo.InitiatorIqn,
	}

	if sh.AuthType == authTypeNone {
		return &shares.PublishContext{
			Protocol:  pc,
			LocalData: pd,
		}, nil
	}

	snip := newTargetcliSnippets(sh.tr)

	err := snip.createClient(ctx, sh.Iqn, iscsiInfo.InitiatorIqn)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	if sh.AuthType == authTypeIqn {
		err = snip.save(ctx)
		if err != nil {
			return nil, fmt.Errorf("save config: %w", err)
		}

		return &shares.PublishContext{
			Protocol:  pc,
			LocalData: pd,
		}, nil
	}

	acl, err := snip.readClientAcl(ctx, sh.Iqn, iscsiInfo.InitiatorIqn)
	if err != nil {
		return nil, fmt.Errorf("read client auth: %w", err)
	}
	if acl == nil {
		logs.FromCtx(ctx).MicroOp("acl is not found, generating a new one", "share-iqn", sh.Iqn, "host-iqn", iscsiInfo.InitiatorIqn)
		acl = &chapAcl{
			userId:         randstr.Base62(16),
			password:       randstr.Base62(32),
			mutualUserId:   randstr.Base62(16),
			mutualPassword: randstr.Base62(32),
		}

		err = snip.setupClientAcl(ctx, sh.Iqn, iscsiInfo.InitiatorIqn, *acl)
		if err != nil {
			return nil, fmt.Errorf("setup client acl: %w", err)
		}
	}

	pc.ClientUser = acl.userId
	pc.ClientPass = acl.password
	pc.TargetUser = acl.mutualUserId
	pc.TargetPass = acl.mutualPassword

	err = snip.save(ctx)
	if err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	return &shares.PublishContext{
		Protocol:  pc,
		LocalData: pd,
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized string) error {
	if !testReady(ctx, sh.tr) {
		return fmt.Errorf("targetcli is not ready")
	}

	pd, err := parsePublishData(serialized)
	if err != nil {
		return fmt.Errorf("parse publish data: %w", err)
	}

	snip := newTargetcliSnippets(sh.tr)

	err = snip.deleteClientAcl(ctx, sh.Iqn, pd.HostIqn)
	if err != nil {
		return fmt.Errorf("delete client acl: %w", err)
	}

	err = snip.save(ctx)
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

func (sh *share) UnpublishAll(ctx context.Context) error {
	return fmt.Errorf("UnpublishAll is not implemented")
}

package shrtargetcli

import (
	"context"
	"fmt"

	"github.com/thanhpk/randstr"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mntriscsiadm"
)

type share struct {
	gp   globalParams
	cmds commands
}

func New(commands map[string][]string, globalParams map[string]string) (shares.ShareHelper, error) {
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	cmds, err := parseCommands(commands)
	if err != nil {
		return nil, fmt.Errorf("parse commands: %w", err)
	}
	return &share{
		gp:   gp,
		cmds: cmds,
	}, nil
}

func (sh *share) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: false,
		LocalVolume:  []string{storage.VolDevice},
		OpaqueVolume: nil,
	}
}

func (sh *share) CreateShare(ctx context.Context, req shares.CreateShareRequest) (string, error) {
	if !sh.cmds.ready(ctx) {
		return "", fmt.Errorf("targetcli is not ready")
	}
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return "", fmt.Errorf("parse params: %w", err)
	}

	devicePath, err := req.Volume.GetLocalPath(ctx)
	if err != nil {
		return "", fmt.Errorf("get local path: %w", err)
	}

	iqn := fmt.Sprintf("%s:%s", sh.gp.iqnBase, req.ShareId)

	snip := newTargetcliSnippets(ctx, sh.cmds.targetcliCmd)

	backstoreName := iqn
	err = snip.configureBackstore(backstoreName, devicePath)
	if err != nil {
		return "", fmt.Errorf("configure backstore: %w", err)
	}

	enforceAuth := true
	disableAuth := false
	switch p.authType {
	case authTypeNone:
		enforceAuth = false
		disableAuth = true
	case authTypeIqn:
		enforceAuth = false
		disableAuth = false
	case authTypeChapProvision:
		enforceAuth = true
		disableAuth = false
	}
	err = snip.configureTarget(iscsiTargetConfig{
		iqn:           iqn,
		backstoreName: backstoreName,
		auth:          enforceAuth,
		demo:          disableAuth,
	})
	if err != nil {
		return "", fmt.Errorf("configure target: %w", err)
	}

	err = snip.save()
	if err != nil {
		return "", fmt.Errorf("save config: %w", err)
	}

	c := shareContext{
		Iqn:           iqn,
		Port:          p.listenPort,
		Transport:     p.transport,
		BackstoreName: backstoreName,
		AuthType:      p.authType,
	}
	err = c.validate()
	if err != nil {
		return "", fmt.Errorf("validate share context: %w", err)
	}

	return c.serialize(), nil
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

	if c.AuthType == authTypeNone {
		return &shares.PublishContext{
			Public: pc,
			Local: map[string]string{
				"host_iqn": iscsiInfo.HostIqn,
			},
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
			Local: map[string]string{
				"host_iqn": iscsiInfo.HostIqn,
			},
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
		Local: map[string]string{
			"host_iqn": iscsiInfo.HostIqn,
		},
		Secret: secret,
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, shareData string, localCtx map[string]string) error {
	if !sh.cmds.ready(ctx) {
		return fmt.Errorf("targetcli is not ready")
	}

	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}

	hostIqn := localCtx["host_iqn"]
	if hostIqn == "" {
		return fmt.Errorf("publish_context: public: field missing: host_iqn")
	}

	snip := newTargetcliSnippets(ctx, sh.cmds.targetcliCmd)

	err = snip.deleteClientAcl(c.Iqn, hostIqn)
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

func (sh *share) DeleteShare(ctx context.Context, shareData string) error {
	if !sh.cmds.ready(ctx) {
		return fmt.Errorf("targetcli is not ready")
	}

	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}

	snip := newTargetcliSnippets(ctx, sh.cmds.targetcliCmd)

	err = snip.deleteTarget(iscsiTargetConfig{
		iqn: c.Iqn,
	})
	if err != nil {
		return fmt.Errorf("delete target: %w", err)
	}
	err = snip.deleteBackstore(c.BackstoreName)
	if err != nil {
		return fmt.Errorf("delete backstore: %w", err)
	}
	err = snip.save()
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

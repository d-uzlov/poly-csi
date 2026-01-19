package shrtargetcli

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
)

type share struct {
	gp   globalParams
	cmds commands
}

func New(commands map[string][]string, globalParams map[string]string) (shares.ShareManager, error) {
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

package shrsamba

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr   transport.Transport
	gp   globalParams
}

func New(tr transport.Transport, globalParams map[string]string) (shares.ShareManager, error) {
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	return &share{
		tr:   tr,
		gp:   gp,
	}, nil
}

func (sh *share) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: true,
		LocalVolume:  []string{storage.VolFolder},
		OpaqueVolume: nil,
	}
}

func (sh *share) CreateShare(ctx context.Context, req shares.CreateShareRequest) (string, error) {
	_, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return "", fmt.Errorf("parse params: %w", err)
	}

	snip := newSambaSnippets(sh.tr, sh.gp)

	shareName := req.ShareId
	userName := fmt.Sprintf("smb-for-%s", req.ShareId)
	userPassword := uuid.NewString()

	err = snip.configureUser(ctx, userName, userPassword)
	if err != nil {
		return "", fmt.Errorf("configure user: %w", err)
	}

	mountPath, err := req.Volume.GetLocalPath(ctx)
	if err != nil {
		return "", fmt.Errorf("get local path: %w", err)
	}

	configFile, err := snip.configureShare(ctx, ShareParameters{
		ShareName: shareName,
		Path:      mountPath,
		User:      userName,
	})
	if err != nil {
		return "", err
	}

	sc := shareContext{
		ConfigFile: configFile,
		ShareName:  shareName,
		UserName:   userName,
		UserPass:   userPassword,
	}
	if sc.validate() != nil {
		return "", fmt.Errorf("validate result: %w", err)
	}
	return sc.serialize(), nil
}

func (sh *share) DeleteShare(ctx context.Context, shareData string) error {
	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}

	snip := newSambaSnippets(sh.tr, sh.gp)

	err = snip.removeShare(ctx, c.ConfigFile)
	if err != nil {
		return fmt.Errorf("remove share config: %w", err)
	}

	err = snip.deleteUser(ctx, c.UserName)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}

package shrsamba

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
)

type manager struct {
	tr transport.Transport
	gp globalParams
}

func New(tr transport.Transport, globalParams map[string]string) (shares.ShareManager, error) {
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	return &manager{
		tr: tr,
		gp: gp,
	}, nil
}

func (m *manager) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: true,
		LocalVolume:  []string{storage.VolFolder},
		OpaqueVolume: nil,
	}
}

func (m *manager) CreateShare(ctx context.Context, req shares.CreateShareRequest) (shares.Share, error) {
	_, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	snip := newSambaSnippets(m.tr, m.gp)

	shareName := req.ShareId
	userName := fmt.Sprintf("smb-for-%s", req.ShareId)
	userPassword := uuid.NewString()

	err = snip.configureUser(ctx, userName, userPassword)
	if err != nil {
		return nil, fmt.Errorf("configure user: %w", err)
	}

	configFile, err := snip.configureShare(ctx, ShareParameters{
		ShareName: shareName,
		Path:      req.FsPath,
		User:      userName,
	})
	if err != nil {
		return nil, err
	}

	sc := share{
		tr:         m.tr,
		gp:         m.gp,
		ConfigFile: configFile,
		ShareName:  shareName,
		UserName:   userName,
		UserPass:   userPassword,
	}
	return &sc, nil
}

func (m *manager) ParseShare(shareData string) (shares.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
		result.gp = m.gp
	}
	return result, err
}

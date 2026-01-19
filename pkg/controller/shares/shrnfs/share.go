package shrnfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
)

type share struct {
	cmds commands
	gp   globalParams
}

func New(commands map[string][]string, globalParams map[string]string) (shares.ShareManager, error) {
	cmds, err := parseCommands(commands)
	if err != nil {
		return nil, fmt.Errorf("parse commands: %w", err)
	}
	gp, err := parseGlobalParams(globalParams)
	if err != nil {
		return nil, fmt.Errorf("parse global params: %w", err)
	}
	return &share{
		cmds: cmds,
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
	p, err := parseParams(req.StaticParams, req.MutableParams)
	if err != nil {
		return "", fmt.Errorf("parse params: %w", err)
	}

	snip := newNfsSnippets(ctx, sh.cmds)

	mountPath, err := req.Volume.GetLocalPath(ctx)
	if err != nil {
		return "", fmt.Errorf("get local path: %w", err)
	}

	token, err := snip.configureNfsShare(nfsShareConfig{
		name:        req.ShareId,
		path:        mountPath,
		globalFlags: p.shareOptions,
		clients: []nfsClientOptions{
			{id: "*"},
		},
	})
	if err != nil {
		return "", err
	}

	return shareContext{
		ExportsFile: token,
		Folder:      mountPath,
	}.serialize(), nil
}

func (sh *share) DeleteShare(ctx context.Context, shareData string) error {
	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}

	snip := newNfsSnippets(ctx, sh.cmds)

	return snip.removeNfsShare(c.ExportsFile)
}

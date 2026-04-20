package shrtnnfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
)

type manager struct {
	tr transport.Transport
}

func New(tr transport.Transport, globalParams map[string]string) (shares.ShareManager, error) {
	return &manager{
		tr: tr,
	}, nil
}

func (sh *manager) GetCapabilities() shares.Capabilities {
	return shares.Capabilities{
		UnpublishAll: true,
		LocalVolume:  []string{storage.VolFolder},
		OpaqueVolume: nil,
	}
}

func (m *manager) CreateShare(ctx context.Context, req shares.CreateShareRequest) (shares.Share, error) {
	snip := newNfsSnippets(m.tr)

	mountPath, err := req.Volume.GetLocalPath(ctx)
	if err != nil {
		return nil, fmt.Errorf("get local path: %w", err)
	}

	err = snip.configureNfsShare(ctx, mountPath)
	if err != nil {
		return nil, err
	}

	c := share{
		tr:     m.tr,
		Folder: mountPath,
	}

	return &c, nil
}

func (m *manager) ParseShare(shareData string) (shares.Share, error) {
	result, err := parseShare(shareData)
	if result != nil {
		result.tr = m.tr
	}
	return result, err
}

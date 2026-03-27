package shrnfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type nfsShareConfig struct {
	name        string
	path        string
	globalFlags string
	clients     []nfsClientOptions
}

type nfsClientOptions struct {
	id    string
	flags string
}

type nfsSnippets struct {
	tr   transport.Transport
}

func newNfsSnippets(tr transport.Transport) nfsSnippets {
	return nfsSnippets{
		tr:   tr,
	}
}

func (s nfsSnippets) configureNfsShare(ctx context.Context, share nfsShareConfig) (string, error) {
	line := share.path
	if share.globalFlags != "" {
		line = fmt.Sprintf("%s -%s", line, share.globalFlags)
	}
	for _, v := range share.clients {
		if v.flags == "" {
			line = fmt.Sprintf("%s %s", line, v.id)
		} else {
			line = fmt.Sprintf("%s %s(%s)", line, v.id, v.flags)
		}
	}
	exportsFile := fmt.Sprintf("/etc/exports.d/%s.exports", share.name)

	err := s.tr.SudoCmds().Write(ctx, exportsFile, []byte(line))
	if err != nil {
		return "", fmt.Errorf("write '%s': %w", exportsFile, err)
	}
	err = transport.CheckedRun(ctx, s.tr, nfsReloadCmd)
	if err != nil {
		return "", fmt.Errorf("nfs reload: %w", err)
	}
	return exportsFile, nil
}

func (s nfsSnippets) removeNfsShare(ctx context.Context, token string) error {
	err := s.tr.SudoCmds().RmFile(ctx, token)
	if err != nil {
		return fmt.Errorf("remove '%s': %w", token, err)
	}
	err = transport.CheckedRun(ctx, s.tr, nfsReloadCmd)
	if err != nil {
		return fmt.Errorf("nfs reload: %w", err)
	}
	return nil
}

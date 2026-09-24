package shrnfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type nfsShareConfig struct {
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

func (s nfsSnippets) configureNfsShare(ctx context.Context, name string, share nfsShareConfig) (error) {
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
	exportsFile := fmt.Sprintf("/etc/exports.d/%s.exports", name)

	err := s.tr.SudoCmds().Write(ctx, exportsFile, []byte(line))
	if err != nil {
		return fmt.Errorf("write '%s': %w", exportsFile, err)
	}
	err = transport.CheckedRun(ctx, s.tr, nfsReloadCmd)
	if err != nil {
		return  fmt.Errorf("nfs reload: %w", err)
	}
	return nil
}

func (s nfsSnippets) removeNfsShare(ctx context.Context, name string) error {
	exportsFile := fmt.Sprintf("/etc/exports.d/%s.exports", name)
	err := s.tr.SudoCmds().RmFile(ctx, exportsFile)
	if err != nil {
		return fmt.Errorf("remove '%s': %w", exportsFile, err)
	}
	err = transport.CheckedRun(ctx, s.tr, nfsReloadCmd)
	if err != nil {
		return fmt.Errorf("nfs reload: %w", err)
	}
	return nil
}

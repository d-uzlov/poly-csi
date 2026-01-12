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
	ctx  context.Context
	cmds commands
}

func newNfsSnippets(ctx context.Context, cmds commands) nfsSnippets {
	return nfsSnippets{
		ctx:  ctx,
		cmds: cmds,
	}
}

func (s nfsSnippets) configureNfsShare(share nfsShareConfig) (string, error) {
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

	tr := transport.FromContext(s.ctx)

	err := tr.SudoCmds().Write(s.ctx, exportsFile, []byte(line))
	if err != nil {
		return "", fmt.Errorf("write '%s': %w", exportsFile, err)
	}
	err = transport.CheckedRun(s.ctx, s.cmds.reloadCmd)
	if err != nil {
		return "", fmt.Errorf("nfs reload: %w", err)
	}
	return exportsFile, nil
}

func (s nfsSnippets) removeNfsShare(token string) error {
	tr := transport.FromContext(s.ctx)
	err := tr.SudoCmds().RmFile(s.ctx, token)
	if err != nil {
		return fmt.Errorf("remove '%s': %w", token, err)
	}
	err = transport.CheckedRun(s.ctx, s.cmds.reloadCmd)
	if err != nil {
		return fmt.Errorf("nfs reload: %w", err)
	}
	return nil
}

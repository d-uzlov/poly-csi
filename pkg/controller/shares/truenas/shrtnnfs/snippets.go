package shrtnnfs

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type tnNfsInfo struct {
	Id   int    `json:"id"`
	Path string `json:"path"`
}

type nfsSnippets struct {
	tr        transport.Transport
	midcltCmd []string
}

func newNfsSnippets(tr transport.Transport, midcltCmd []string) nfsSnippets {
	return nfsSnippets{
		tr:        tr,
		midcltCmd: midcltCmd,
	}
}

func (s nfsSnippets) configureNfsShare(ctx context.Context, path string) error {
	shareList, err := s.listNfsSharesForPath(ctx, path)
	if err != nil {
		return nil
	}

	if len(shareList) != 0 {
		return nil
	}

	cmd := append(s.midcltCmd, "call", "sharing.nfs.create", fmt.Sprintf(`{"path": "%s"}`, path))
	stdout, err := transport.CheckedRunStdout(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	var share tnNfsInfo
	err = json.Unmarshal([]byte(stdout), &share)
	if err != nil {
		return err
	}

	return nil
}

func (s nfsSnippets) listNfsSharesForPath(ctx context.Context, path string) ([]tnNfsInfo, error) {
	cmd := append(s.midcltCmd, "call", "sharing.nfs.query", fmt.Sprintf(`[["path","=","%s"]]`, path))
	stdout, err := transport.CheckedRunStdout(ctx, s.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}

	var shareList []tnNfsInfo
	err = json.Unmarshal([]byte(stdout), &shareList)
	if err != nil {
		return nil, err
	}

	return shareList, nil
}

func (s nfsSnippets) removeNfsShare(ctx context.Context, path string) error {
	shareList, err := s.listNfsSharesForPath(ctx, path)
	if err != nil {
		return nil
	}

	if len(shareList) == 0 {
		return nil
	}

	for _, v := range shareList {
		cmd := append(s.midcltCmd, "call", "sharing.nfs.delete", fmt.Sprintf("%d", v.Id))
		err := transport.CheckedRun(ctx, s.tr, cmd)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
	}

	return nil
}

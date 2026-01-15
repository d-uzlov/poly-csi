package btrfs

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type FilesystemDfEntry struct {
	BgType    string `json:"bg-type"`
	BgProfile string `json:"bg-profile"`
	Total     string `json:"total"`
	Used      string `json:"used"`
}

func (s *Snippets) FilesystemUsage(ctx context.Context, path string) error {
	cmd := append(s.btrfsCmd, "--format=json", "filesystem", "df", path)
	err := transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

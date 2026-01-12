package strfolder

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

func createFolder(ctx context.Context, path string) error {
	tr := transport.FromContext(ctx)
	// TODO customize sudo
	return tr.SudoCmds().Mkdir(ctx, path, true)
}

func deleteFolder(ctx context.Context, path string) error {
	tr := transport.FromContext(ctx)
	err := tr.SudoCmds().RmDirRecursive(ctx, path)
	if err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func getAvailableCapacity(ctx context.Context, path string) (int64, error) {
	cmd := []string{"df", "--output=avail", "--block-size=1", path}
	// cmd := fmt.Sprintf("df --output=avail --block-size=1 '%s' | tail -n +2", path)
	stdout, err := transport.CheckedRunStdout(ctx, cmd)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", cmd, err)
	}
	lines := strings.Split(stdout, "\n")
	if len(lines) != 2 {
		return 0, fmt.Errorf("expected only 2 lines in df stdout: %s", stdout)
	}

	result, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return result, nil
}

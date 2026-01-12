package fsutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	slogctx "github.com/veqryn/slog-context"
	"k8s.io/mount-utils"
	"meoe.io/poly-csi/pkg/transport"
)

type FsInfo struct {
	Uuid      string `json:"uuid"`
	BlockSize string `json:"block_size"`
	FsType    string `json:"type"`
}

func GetBlkid(ctx context.Context, devicePath string) (*FsInfo, error) {
	// can't use '-o json' here because busybox does not support it
	cmd := []string{"blkid", devicePath}
	tr := transport.FromContext(ctx)
	runResult, err := tr.RunCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("command failed: %w", err)
	}
	// on debian blkid gives exit code 2 when device is not formatted
	if runResult.ExitCode == 2 {
		return nil, nil
	}
	if runResult.ExitCode != 0 {
		err := transport.FormatRunError(runResult)
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}
	// blkid from busybox gives exit code 0, but empty output when device is not formatted
	if string(runResult.Stdout) == "" {
		return nil, nil
	}
	prefix := fmt.Sprintf("%s:", devicePath)
	stdout, ok := strings.CutPrefix(string(runResult.Stdout), prefix)
	if !ok {
		return nil, fmt.Errorf("blkid: expected stdout to have prefix '%s', stdout: %s", prefix, stdout)
	}
	var result FsInfo
	for _, v := range strings.Fields(stdout) {
		vnl := strings.SplitN(v, "=", 2)
		if len(vnl) < 2 {
			continue
		}
		switch vnl[0] {
		case "UUID":
			result.Uuid = strings.Trim(vnl[1], "\"")
		case "TYPE":
			result.FsType = strings.Trim(vnl[1], "\"")
		case "BLOCK_SIZE":
			result.BlockSize = strings.Trim(vnl[1], "\"")
		}
	}
	return &result, nil
}

func EnsureMounted(sourcePath string, targetPath string, fsType string, mountOptions []string) (*mount.MountInfo, error) {
	err := os.MkdirAll(targetPath, 0777)
	if err != nil {
		return nil, fmt.Errorf("create target path: %w", err)
	}

	// if mountpoint already exists, `mount` will create a new mountpoint in the same place, without altering the first one
	// let's check if mountpoint is already mounted, to avoid this
	mi, err := findMount(targetPath)
	if err != nil {
		return nil, err
	}
	if mi != nil {
		return mi, nil
	}
	// we didn't find a matching mountpoint, proceed to mount

	err = os.MkdirAll(targetPath, 0777)
	if err != nil {
		return nil, fmt.Errorf("create target path: %w", err)
	}
	m := mount.NewWithoutSystemd("")

	err = m.Mount(sourcePath, targetPath, fsType, mountOptions)
	if err != nil {
		return nil, fmt.Errorf("mount: %w", err)
	}
	mi, err = findMount(targetPath)
	if err != nil {
		return nil, err
	}
	if mi == nil {
		return nil, fmt.Errorf("mount succeeded but mount did not appear in list of mounts")
	}
	return mi, nil
}

func findMount(mountedPath string) (*mount.MountInfo, error) {
	realPath, err := filepath.EvalSymlinks(mountedPath)
	if err != nil {
		return nil, fmt.Errorf("EvalSymlinks: %w", err)
	}
	mis, err := mount.ParseMountInfo("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	for _, i := range mis {
		if i.MountPoint == realPath {
			return &i, nil
		}
	}
	return nil, nil
}

func FormatFs(ctx context.Context, devicePath string, fsType string, formatOptions []string) error {
	switch fsType {
	case "ext4":
		// TODO use format options
		cmd := []string{"mkfs.ext4", devicePath}
		err := transport.CheckedRun(ctx, cmd)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		return nil
	}
	return fmt.Errorf("filesystem %s is not supported", fsType)
}

func ExpandFs(ctx context.Context, mountedPath string) error {
	mi, err := findMount(mountedPath)
	if err != nil {
		return err
	}

	slogctx.FromCtx(ctx).Info("ExpandFs", "mi.FsType", mi.FsType)
	switch mi.FsType {
	case "ext4":
		cmd := []string{"resize2fs", mi.Source}
		err := transport.CheckedRun(ctx, cmd)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		return nil
	}
	return fmt.Errorf("filesystem %s is not supported", mi.FsType)
}

func EnsureBindMount(sourcePath string, targetPath string) error {
	err := os.MkdirAll(targetPath, 0777)
	if err != nil {
		return fmt.Errorf("create target path: %w", err)
	}

	m := mount.NewWithoutSystemd("")

	isMount, err := m.IsMountPoint(targetPath)
	if err != nil {
		return fmt.Errorf("IsMountPoint: %w", err)
	}
	if isMount {
		// target is already mounted, nothing to do
		return nil
	}

	err = m.Mount(sourcePath, targetPath, "", []string{"bind"})
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}

	return nil
}

func Unmount(ctx context.Context, targetPath string) error {
	cmd := []string{"umount", "-r", "-l", "-f", "-d", targetPath}
	_, err := transport.RelaxedRun(ctx, cmd, "Invalid argument")
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

type StatsResult struct {
	Capacity        int64
	Used            int64
	Available       int64
	InodesCapacity  int64
	InodesUsed      int64
	InodesAvailable int64
}

func GetMountStats(targetPath string) (StatsResult, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(targetPath, &stat)
	if err != nil {
		return StatsResult{}, err
	}
	return StatsResult{
		Capacity:        int64(stat.Bsize) * int64(stat.Blocks),
		Used:            int64(stat.Bsize) * int64(stat.Blocks-stat.Bfree),
		Available:       int64(stat.Bsize) * int64(stat.Bavail),
		InodesCapacity:  int64(stat.Files),
		InodesAvailable: int64(stat.Ffree),
		InodesUsed:      int64(stat.Files) - int64(stat.Ffree),
	}, nil
}

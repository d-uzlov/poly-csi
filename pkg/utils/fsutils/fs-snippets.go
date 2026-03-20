package fsutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"k8s.io/mount-utils"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
)

type FsSnippets struct {
	tr        transport.Transport
	devFolder string
}

func New(tr transport.Transport, devFolder string) *FsSnippets {
	return &FsSnippets{
		tr:        tr,
		devFolder: devFolder,
	}
}

type FsInfo struct {
	Uuid      string `json:"uuid"`
	BlockSize string `json:"block_size"`
	FsType    string `json:"type"`
}

func (s *FsSnippets) GetBlkid(ctx context.Context, devicePath string) (*FsInfo, error) {
	// can't use '-o json' here because busybox does not support it
	cmd := []string{"blkid", devicePath}
	runResult, err := s.tr.RunCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("blkid failed: %w", err)
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

func (s *FsSnippets) EnsureMounted(ctx context.Context, sourcePath string, targetPath string, fsType string, mountOptions []string) (*mount.MountInfo, error) {
	err := os.MkdirAll(targetPath, 0777)
	if err != nil {
		return nil, fmt.Errorf("create target path: %w", err)
	}

	// if mountpoint already exists, `mount` will create a new mountpoint in the same place, without altering the first one
	// let's check if mountpoint is already mounted, to avoid this
	mi, err := s.findMount(targetPath)
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
	cmd := []string{"mount"}
	if fsType != "" {
		cmd = append(cmd, "-t", fsType)
	}
	for _, v := range mountOptions {
		cmd = append(cmd, "-o", v)
	}
	// can't use --source and --target because busybox does not support them
	cmd = append(cmd, sourcePath, targetPath)

	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("mount: %w", err)
	}

	mi, err = s.findMount(targetPath)
	if err != nil {
		return nil, err
	}
	if mi == nil {
		return nil, fmt.Errorf("mount succeeded but mount did not appear in list of mounts")
	}
	return mi, nil
}

func (s *FsSnippets) findMount(mountedPath string) (*mount.MountInfo, error) {
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

func (s *FsSnippets) FormatFs(ctx context.Context, devicePath string, fsType string, formatOptions []string) error {
	switch fsType {
	// TODO more filesystem options
	case "ext4":
		// TODO use format options
		cmd := []string{"mkfs.ext4", devicePath}
		err := transport.CheckedRun(ctx, s.tr, cmd)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		return nil
	}
	return fmt.Errorf("filesystem %s is not supported", fsType)
}

func (s *FsSnippets) findBlockDevice(ctx context.Context, minMaj string) (string, error) {
	link := fmt.Sprintf("/sys/dev/block/%s/", minMaj)
	sr, err := s.tr.Cmds().Stat(ctx, link)
	if err != nil {
		return "", fmt.Errorf("failed to get device link: %w", err)
	}
	if !sr.FileExists {
		return "", fmt.Errorf("requested device does not exist")
	}
	if !sr.IsDirectory() {
		return "", fmt.Errorf("file is not a directory:  %v", sr.Type)
	}
	return filepath.Base(sr.Path), nil
}

func (s *FsSnippets) ExpandFs(ctx context.Context, mountedPath string) error {
	mi, err := s.findMount(mountedPath)
	if err != nil {
		return err
	}

	minMaj := fmt.Sprintf("%d:%d", mi.Major, mi.Minor)
	deviceName, err := s.findBlockDevice(ctx, minMaj)
	devicePath := fmt.Sprintf("%s/%s", s.devFolder, deviceName)

	logs.FromCtx(ctx).Info("ExpandFs", "mi.FsType", mi.FsType)
	switch mi.FsType {
	case "ext4":
		cmd := []string{"resize2fs", devicePath}
		err := transport.CheckedRun(ctx, s.tr, cmd)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		return nil
	}
	return fmt.Errorf("filesystem %s is not supported", mi.FsType)
}

func (s *FsSnippets) EnsureBindMount(ctx context.Context, sourcePath string, targetPath string) error {
	err := os.MkdirAll(targetPath, 0777)
	if err != nil {
		return fmt.Errorf("create target path: %w", err)
	}

	mi, err := s.findMount(targetPath)
	if err != nil {
		return err
	}
	if mi != nil {
		// target is already mounted, nothing to do
		return nil
	}
	cmd := []string{"mount", "--bind", sourcePath, targetPath}
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}

	return nil
}

func (s *FsSnippets) Unmount(ctx context.Context, targetPath string) error {
	mi, err := s.findMount(targetPath)
	if err != nil {
		return fmt.Errorf("check existing mountpoint: %w", err)
	}
	if mi == nil {
		return nil
	}

	// from umount --help:
	// -r      Remount devices read-only if mount is busy
	// -l      Lazy umount (detach filesystem)
	// -f      Force umount (i.e., unreachable NFS server)
	// can't use proper full-word options because busybox does not support them
	cmd := []string{"umount", "-r", targetPath}
	runResult, err := s.tr.RunCommand(ctx, cmd, transport.WithTimeout(time.Second))
	if err != nil {
		return fmt.Errorf("unmount 1: %w", cmd, err)
	}
	if runResult.ExitCode == 0 {
		return nil
	}

	logs.FromCtx(ctx).Warn("umount failed, will try 'umount --force'", "error", transport.FormatRunError(runResult))
	cmd = []string{"umount", "-r", "-f", targetPath}
	runResult, err = s.tr.RunCommand(ctx, cmd, transport.WithTimeout(time.Second))
	if err != nil {
		return fmt.Errorf("unmount 2: %w", cmd, err)
	}
	if runResult.ExitCode == 0 {
		return nil
	}

	logs.FromCtx(ctx).Warn("umount --force failed, will try 'umount --force --lazy'", "error", transport.FormatRunError(runResult))
	cmd = []string{"umount", "-r", "-f", "-l", targetPath}
	runResult, err = s.tr.RunCommand(ctx, cmd, transport.WithTimeout(time.Second))
	if err != nil {
		return fmt.Errorf("unmount 3: %w", cmd, err)
	}
	if runResult.ExitCode == 0 {
		return nil
	}

	return fmt.Errorf("failed to unmount via 'umount --force --lazy': %w", transport.FormatRunError(runResult))
}

type StatsResult struct {
	Capacity        int64
	Used            int64
	Available       int64
	InodesCapacity  int64
	InodesUsed      int64
	InodesAvailable int64
}

func (s *FsSnippets) GetMountStats(targetPath string) (StatsResult, error) {
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

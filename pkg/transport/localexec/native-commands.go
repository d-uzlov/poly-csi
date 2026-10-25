package localexec

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/executils"
)

type nativeCommands struct {
	tr transport.Transport
}

func newNativeCommands(tr transport.Transport) *nativeCommands {
	return &nativeCommands{
		tr: tr,
	}
}

func (s *nativeCommands) Read(ctx context.Context, path string, trim bool) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("read %s", path))

	result, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if trim {
		result = []byte(strings.TrimSpace(string(result)))
	}
	return result, nil
}

func (s *nativeCommands) Write(ctx context.Context, path string, data []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("write %s", path))

	return os.WriteFile(path, data, 0666)
}

func (s *nativeCommands) WriteMove(ctx context.Context, path string, data []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("write move %s", path))

	tmpFile := path + ".~tmp"

	err := os.WriteFile(tmpFile, data, 0666)
	if err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	err = os.Rename(tmpFile, path)
	if err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

func (s *nativeCommands) Ls(ctx context.Context, path string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("ls %s", path))

	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("os.ReadDir: %w", err)
	}
	var result []string
	for _, v := range entries {
		result = append(result, v.Name())
	}
	return result, nil
}

func (s *nativeCommands) Move(ctx context.Context, source string, dest string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if source == dest {
		return nil
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("move '%s' '%s'", source, dest))

	err := os.Rename(source, dest)
	// todo handle moves between filesystems
	return err
}

func (s *nativeCommands) Mkdir(ctx context.Context, path string, parents bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if parents {
		logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("mkdir with parents %s", path))

		return os.MkdirAll(path, 0777)
	}
	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("mkdir %s", path))

	err := os.Mkdir(path, 0777)
	if !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}

func (s *nativeCommands) RmDirRecursive(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("rmdir recursive %s", path))

	return os.RemoveAll(path)
}

func (s *nativeCommands) RmDir(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("rmdir %s", path))

	err := os.Remove(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func (s *nativeCommands) RmFile(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("rm %s", path))

	err := os.Remove(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func (s *nativeCommands) ReadLink(ctx context.Context, path string) (result string, err error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("readlink %s", path))
	defer func() {
		logs.FromCtx(ctx).Result("local exec: running command (optimized)", "cmd", fmt.Sprintf("readlink %s", path), "result", result, "error", err)
	}()

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	parent := filepath.Dir(abs)
	filename := filepath.Base(abs)

	resolvedDir, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", fmt.Errorf("filepath.EvalSymlinks: %w", err)
	}

	info, err := os.Stat(resolvedDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("os.Stat(parentDir): %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("parent is not a directory: %s", resolvedDir)
	}

	// os.Readlink does not like it when file is not a symlink, need to check it first
	file := filepath.Join(resolvedDir, filename)
	info, err = os.Lstat(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return file, nil
		}
		return "", fmt.Errorf("os.Stat(file): %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return file, nil
	}

	resolvedLink, err := os.Readlink(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return file, nil
		}
		return "", fmt.Errorf("os.Readlink: %w", err)
	}
	if filepath.IsAbs(resolvedLink) {
		return resolvedLink, nil
	}

	resolvedAbs, err := filepath.Abs(filepath.Join(resolvedDir, resolvedLink))
	if err != nil {
		return "", err
	}
	return resolvedAbs, nil
}

func (s *nativeCommands) Stat(ctx context.Context, path string) (result transport.StatResult, err error) {
	if ctx.Err() != nil {
		return transport.StatResult{}, ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("stat %s", path))
	defer func() {
		logs.FromCtx(ctx).Result("local exec: running command (optimized)", "cmd", fmt.Sprintf("stat %s", path), "result", result, "error", err)
	}()

	realPath, err := s.ReadLink(ctx, path)
	if err != nil {
		return transport.StatResult{}, fmt.Errorf("ReadLink: %w", err)
	}
	if realPath == "" {
		return transport.StatResult{
			Type:         0,
			Path:         "",
			FileExists:   false,
			ParentExists: false,
		}, nil
	}

	stat, err := os.Stat(realPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return transport.StatResult{
				Type:         0,
				Path:         realPath,
				FileExists:   false,
				ParentExists: true,
			}, nil
		}
		return transport.StatResult{}, fmt.Errorf("os.Stat: %w", err)
	}
	sysStat, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return transport.StatResult{}, fmt.Errorf("os.Stat: not a posix filesystem")
	}

	result = transport.StatResult{
		Path:         realPath,
		FileExists:   true,
		ParentExists: true,
		Uid:          fmt.Sprintf("%d", sysStat.Uid),
		Gid:          fmt.Sprintf("%d", sysStat.Gid),
	}

	mode := stat.Mode()
	switch {
	case mode.IsDir():
		result.Type = transport.FtDirectory
	case mode.IsRegular():
		result.Type = transport.FtFile
	case mode&os.ModeDevice != 0:
		result.Type = transport.FtBlock
	case mode&os.ModeNamedPipe != 0:
		result.Type = transport.FtFifo
	}

	return result, nil
}

func (s *nativeCommands) Df(ctx context.Context, path string) (*transport.FsSizeInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	// TODO implement df natively
	return executils.New(s.tr, false).Df(ctx, path)
}

func (s *nativeCommands) EnsureSymlink(ctx context.Context, link string, target string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if link == target {
		return nil
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("link '%s' -> '%s'", link, target))

	sr, err := s.Stat(ctx, link)
	if err != nil {
		return fmt.Errorf("check existing link at '%s': %w", link, err)
	}
	if sr.FileExists {
		if sr.Path == link {
			return fmt.Errorf("path '%s' already contains non-link", link)
		}
		if sr.Path != target {
			return fmt.Errorf("existing link '%s' points to '%s'", link, sr.Path)
		}
		// link already points to required path
		return nil
	}
	// link does not exist, need to create it
	err = os.Symlink(target, link)
	if err != nil {
		return fmt.Errorf("os.Symlink: %w", err)
	}
	return nil
}

func (s *nativeCommands) MkFifo(ctx context.Context, path string) (alreadyExists bool, err error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("mkfifo %s", path))
	err = syscall.Mkfifo(path, 0666)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

func (s *nativeCommands) CreateLock(ctx context.Context, lockFile string, maxDuration time.Duration) (func(ctx context.Context) error, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	// TODO add optimized implementation
	return executils.New(s.tr, false).CreateLock(ctx, lockFile, maxDuration)
}

func (s *nativeCommands) Chown(ctx context.Context, path string, user_ string, group string, recursive bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	uid := -1
	gid := -1
	var err error

	if user_ != "" {
		uid, err = lookupUid(user_)
		if err != nil {
			return fmt.Errorf("resolve user '%s': %w", user_, err)
		}
	}

	if group != "" {
		gid, err = lookupGid(group)
		if err != nil {
			return fmt.Errorf("resolve group '%s': %w", group, err)
		}
	}

	logs.FromCtx(ctx).Command("local exec: os.Chown", "path", path, "uid", uid, "gid", gid, "recursive", recursive)

	if !recursive {
		return os.Chown(path, uid, gid)
	}

	return filepath.WalkDir(path, func(curPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(curPath, uid, gid)
	})
}

func lookupUid(str string) (int, error) {
	if val, err := strconv.Atoi(str); err == nil {
		return val, nil
	}
	u, err := user.Lookup(str)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(u.Uid)
}
func lookupGid(str string) (int, error) {
	if val, err := strconv.Atoi(str); err == nil {
		return val, nil
	}
	g, err := user.LookupGroup(str)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(g.Gid)
}

func (s *nativeCommands) Chmod(ctx context.Context, path string, mode string, recursive bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	modeBits, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("parse mode: %w", err)
	}
	mode32 := os.FileMode(modeBits)

	logs.FromCtx(ctx).Command("local exec: os.Chmod", "path", path, "mode", strconv.FormatUint(uint64(mode32), 8), "recursive", recursive)

	if !recursive {
		return os.Chmod(path, mode32)
	}
	return filepath.WalkDir(path, func(curPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chmod(path, mode32)
	})
}

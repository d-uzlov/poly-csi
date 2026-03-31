package localexec

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

func (s *nativeCommands) ReadLink(ctx context.Context, path string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("readlink %s", path))

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
	info, err = os.Stat(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return file, nil
		}
		return "", fmt.Errorf("os.Stat(file): %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return file, nil
	}

	resolvedFile, err := os.Readlink(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return file, nil
		}
		return "", fmt.Errorf("os.Readlink: %w", err)
	}
	return resolvedFile, nil
}

func (s *nativeCommands) Stat(ctx context.Context, path string) (transport.StatResult, error) {
	if ctx.Err() != nil {
		return transport.StatResult{}, ctx.Err()
	}

	logs.FromCtx(ctx).Command("local exec: running command (optimized)", "cmd", fmt.Sprintf("stat %s", path))

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

	result := transport.StatResult{
		Path:         realPath,
		FileExists:   true,
		ParentExists: true,
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
	// TODO add optimized implementation for CreateLock
	return executils.New(s.tr, false).CreateLock(ctx, lockFile, maxDuration)
}

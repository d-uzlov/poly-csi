package localexec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/executils"
)

type nativeCommands struct {
	tr         transport.Transport
	enableEcho bool
}

func newNativeCommands(tr transport.Transport, enableEcho bool) *nativeCommands {
	return &nativeCommands{
		tr:         tr,
		enableEcho: enableEcho,
	}
}

func (s *nativeCommands) Read(ctx context.Context, path string, trim bool) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if s.enableEcho {
		slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("read %s", path))
	}
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

	if s.enableEcho {
		slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("write %s", path))
	}

	return os.WriteFile(path, data, 0644)
}

func (s *nativeCommands) Ls(ctx context.Context, path string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if s.enableEcho {
		slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("ls %s", path))
	}

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

	if s.enableEcho {
		slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("move '%s' '%s'", source, dest))
	}

	err := os.Rename(source, dest)
	// todo handle moves between filesystems
	return err
}

func (s *nativeCommands) Mkdir(ctx context.Context, path string, parents bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if parents {
		if s.enableEcho {
			slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("mkdir with parents %s", path))
		}
		return os.MkdirAll(path, 0755)
	}
	if s.enableEcho {
		slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("mkdir %s", path))
	}
	err := os.Mkdir(path, 0755)
	if !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}

func (s *nativeCommands) RmDirRecursive(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if s.enableEcho {
		slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("rmdir recursive %s", path))
	}
	return os.RemoveAll(path)
}

func (s *nativeCommands) RmDir(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if s.enableEcho {
		slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("rmdir %s", path))
	}

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

	if s.enableEcho {
		slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("rm %s", path))
	}
	err := os.Remove(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func (s *nativeCommands) Stat(ctx context.Context, path string) (transport.StatResult, error) {
	if ctx.Err() != nil {
		return transport.StatResult{}, ctx.Err()
	}
	// TODO use os.Stat instead of cmds fallback
	return executils.New(s.tr, false).Stat(ctx, path)
}

func (s *nativeCommands) Df(ctx context.Context, path string) (*transport.FsSizeInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	// TODO implement df natively
	return executils.New(s.tr, false).Df(ctx, path)
}

func (s *nativeCommands) CreateSymlink(ctx context.Context, link string, target string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if s.enableEcho {
		slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("create symlink '%s' -> '%s'", link, target))
	}
	err := os.Symlink(target, link)
	if err != nil {
		return err
	}

	return nil
}

func (s *nativeCommands) MkFifo(ctx context.Context, path string) (alreadyExists bool, err error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if s.enableEcho {
		slogctx.FromCtx(ctx).Debug("local exec: running command (optimized)", "cmd", fmt.Sprintf("mkfifo %s", path))
	}
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

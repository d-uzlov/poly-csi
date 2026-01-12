package jsonfiledb

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type Entry struct {
	Directory string
}

func (e Entry) DeleteEntry(ctx context.Context) error {
	cmds := transport.FromContext(ctx).Cmds()

	err := cmds.RmDirRecursive(ctx, e.Directory)
	if err != nil {
		return fmt.Errorf("remove %s: %w", e.Directory, err)
	}
	return nil
}

// CreateUnsafe should be called after acquiring global lock
func (e Entry) CreateUnsafe(ctx context.Context) error {
	cmds := transport.FromContext(ctx).Cmds()

	err := cmds.Mkdir(ctx, e.Directory, true)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", e.Directory, err)
	}

	return nil
}

func (e Entry) CheckExists(ctx context.Context) error {
	cmds := transport.FromContext(ctx).Cmds()

	sr, err := cmds.Stat(ctx, e.Directory)
	if err != nil {
		return fmt.Errorf("stat %s: %w", e.Directory, err)
	}
	if sr == nil {
		return fmt.Errorf("stat %s: directory is not found", e.Directory)
	}
	if !sr.IsDirectory() {
		return fmt.Errorf("path '%s' is not a directory", e.Directory)
	}

	return nil
}

func (e Entry) ListModules(ctx context.Context) ([]string, error) {
	cmds := transport.FromContext(ctx).Cmds()

	modules, err := cmds.Ls(ctx, e.Directory)
	if err != nil {
		return nil, fmt.Errorf("ls %s: %w", e.Directory, err)
	}

	return modules, nil
}

func (e Entry) SetModule(ctx context.Context, module string, data []byte) error {
	cmds := transport.FromContext(ctx).Cmds()
	if !validateId(module) {
		return fmt.Errorf("module name '%s' is invalid", module)
	}

	path := fmt.Sprintf("%s/%s", e.Directory, module)
	err := cmds.Write(ctx, path, data)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func (e Entry) GetModule(ctx context.Context, module string) ([]byte, error) {
	cmds := transport.FromContext(ctx).Cmds()
	if !validateId(module) {
		return nil, fmt.Errorf("module name '%s' is invalid", module)
	}

	path := fmt.Sprintf("%s/%s", e.Directory, module)

	content, err := cmds.Read(ctx, path, false)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, nil
	}
	return content, nil
}

func (e Entry) DeleteModule(ctx context.Context, module string) error {
	cmds := transport.FromContext(ctx).Cmds()
	if !validateId(module) {
		return fmt.Errorf("module name '%s' is invalid", module)
	}

	path := fmt.Sprintf("%s/%s", e.Directory, module)
	err := cmds.RmFile(ctx, path)
	if err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}

	return nil
}

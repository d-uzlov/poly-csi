package jsonfiledb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"meoe.io/poly-csi/pkg/transport"
)

type Entry struct {
	directory string
	uuid      string
}

const (
	uuidFile = "@uuid"
	lockFile = "@lock"
)

func newEntry(directory string, uuid string) *Entry {
	return &Entry{
		directory: directory,
		uuid:      strings.TrimSpace(uuid),
	}
}

func (e Entry) DeleteEntry(ctx context.Context) error {
	cmds := transport.FromContext(ctx).Cmds()

	err := cmds.RmDirRecursive(ctx, e.directory)
	if err != nil {
		return fmt.Errorf("remove %s: %w", e.directory, err)
	}
	return nil
}

// unsafeCreate should be called after acquiring global lock
func (e Entry) unsafeCreate(ctx context.Context) error {
	cmds := transport.FromContext(ctx).Cmds()

	err := cmds.Mkdir(ctx, e.directory, true)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", e.directory, err)
	}
	uuidPath := fmt.Sprintf("%s/%s", e.directory, uuidFile)
	err = cmds.Write(ctx, uuidPath, []byte(e.uuid))
	if err != nil {
		return fmt.Errorf("write uuid to %s: %w", uuidPath, err)
	}

	return nil
}

func (e Entry) readUuid(ctx context.Context) (string, error) {
	uuidPath := fmt.Sprintf("%s/%s", e.directory, uuidFile)
	cmds := transport.FromContext(ctx).Cmds()
	uuid, err := cmds.Read(ctx, uuidPath, false)
	if err != nil {
		return "", err
	}
	return string(uuid), nil
}

func (e Entry) fillUuid(ctx context.Context) error {
	if e.uuid != "" {
		return nil
	}
	uuid, err := e.readUuid(ctx)
	if err != nil {
		return err
	}
	e.uuid = uuid
	return nil
}

func (e Entry) Lock(ctx context.Context) (release func(ctx context.Context) error, err error) {
	lockPath := fmt.Sprintf("%s/%s", e.directory, lockFile)
	cmds := transport.FromContext(ctx).Cmds()
	// TODO make timeout infinite?
	release, err = cmds.CreateLock(ctx, lockPath, time.Minute)
	return release, err
}

func (e Entry) CheckValid(ctx context.Context) error {
	if e.uuid == "" {
		return fmt.Errorf("entry is not initialized with uuid")
	}

	cmds := transport.FromContext(ctx).Cmds()

	sr, err := cmds.Stat(ctx, e.directory)
	if err != nil {
		return fmt.Errorf("stat %s: %w", e.directory, err)
	}
	if sr == nil {
		return fmt.Errorf("stat %s: directory is not found", e.directory)
	}
	if !sr.IsDirectory() {
		return fmt.Errorf("path '%s' is not a directory", e.directory)
	}

	currentUuid, err := e.readUuid(ctx)
	if err != nil {
		return err
	}
	if e.uuid != currentUuid {
		return fmt.Errorf("uuid on disk has changed: expected %s != current %s", e.uuid, currentUuid)
	}

	return nil
}

func (e Entry) ListModules(ctx context.Context) ([]string, error) {
	cmds := transport.FromContext(ctx).Cmds()

	modules, err := cmds.Ls(ctx, e.directory)
	if err != nil {
		return nil, fmt.Errorf("ls %s: %w", e.directory, err)
	}

	return modules, nil
}

func (e Entry) SetModule(ctx context.Context, module string, data []byte) error {
	cmds := transport.FromContext(ctx).Cmds()
	if !validateId(module) {
		return fmt.Errorf("module name '%s' is invalid", module)
	}

	path := fmt.Sprintf("%s/%s", e.directory, module)
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

	path := fmt.Sprintf("%s/%s", e.directory, module)

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

	path := fmt.Sprintf("%s/%s", e.directory, module)
	err := cmds.RmFile(ctx, path)
	if err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}

	return nil
}

package jsonfiledb

import (
	"context"
	"fmt"
	"strings"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/controller/voldb"
	"meoe.io/poly-csi/pkg/transport"
)

type JsonFileDatabase struct {
	config JsonFileInfoManagerConfig
}

func NewJsonFileVolumeDatabase(config JsonFileInfoManagerConfig) voldb.VolumeDatabase {
	return &JsonFileDatabase{config: config}
}

func validateId(id string) bool {
	const charset = ":/"
	return !strings.ContainsAny(id, charset)
}

func (m *JsonFileDatabase) Cleanup(ctx context.Context) error {
	return nil
}

func (m *JsonFileDatabase) CreateEntry(ctx context.Context, name string) (voldb.Entry, error) {
	if !validateId(name) {
		return nil, fmt.Errorf("entry name '%s' is invalid", name)
	}

	cmds := transport.FromContext(ctx).Cmds()

	globalLock := fmt.Sprintf("%s/entries.lock", m.config.Directory)

	release, err := cmds.CreateLock(ctx, globalLock, time.Minute)
	if err != nil {
		return nil, fmt.Errorf("createLock: %w", err)
	}
	// TODO use different context for this
	defer func() {
		releaseErr := release(ctx)
		if releaseErr != nil {
			slogctx.FromCtx(ctx).Error("failed to release lock", "error", releaseErr)
		}
	}()

	folder := fmt.Sprintf("%s/%s/", m.config.Directory, name)
	entry := Entry{Directory: folder}
	err = entry.CreateUnsafe(ctx)
	if err != nil {
		return nil, fmt.Errorf("create entry in filesystem: %w", err)
	}

	return &entry, nil
}

func (m *JsonFileDatabase) GetEntry(ctx context.Context, name string) (voldb.Entry, error) {
	if !validateId(name) {
		return nil, fmt.Errorf("entry name '%s' is invalid", name)
	}

	folder := fmt.Sprintf("%s/%s/", m.config.Directory, name)
	entry := Entry{Directory: folder}
	err := entry.CheckExists(ctx)
	if err != nil {
		return nil, nil
	}

	return &entry, nil
}

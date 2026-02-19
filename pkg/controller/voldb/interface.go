package voldb

import (
	"context"
)

type VolumeDatabase interface {
	// Cleanup disposes of this VolumeDatabase instance.
	// It does not affect any entries.
	Cleanup() error
	// CreateEntry fetches existing entry, or creates a new entry if necessary.
	CreateEntry(ctx context.Context, name string, uuid string) (Entry, error)
	// FindEntry fetches existing entry, or returns an error if entry does not exist.
	FindEntry(ctx context.Context, name string) (Entry, error)
}

type Entry interface {
	Lock(ctx context.Context) (release func(ctx context.Context) error, err error)
	DeleteEntry(ctx context.Context) error
	ListModules(ctx context.Context) ([]string, error)
	SetModule(ctx context.Context, module string, data []byte) error
	GetModule(ctx context.Context, module string) ([]byte, error)
	DeleteModule(ctx context.Context, module string) error
}

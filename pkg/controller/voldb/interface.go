package voldb

import (
	"context"
)

type VolumeDatabase interface {
	Cleanup(ctx context.Context) error
	// CreateEntry creates a new entry, and returns handle to it.
	CreateEntry(ctx context.Context, name string) (Entry, error)
	// GetEntry creates entry handle if entry already exists in the underlying storage
	GetEntry(ctx context.Context, name string) (Entry, error)
}

type Entry interface {
	DeleteEntry(ctx context.Context) (error)
	ListModules(ctx context.Context) ([]string, error)
	SetModule(ctx context.Context, module string, data []byte) error
	GetModule(ctx context.Context, module string) ([]byte, error)
	DeleteModule(ctx context.Context, module string) error
}

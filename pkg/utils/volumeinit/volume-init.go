package volumeinit

import (
	"context"

	"meoe.io/poly-csi/pkg/transport"
)

const PreferredPrefix = "init."

type FilesystemHelper interface {
	Configure(params map[string]string, props StorageProperties) error
	Format(ctx context.Context, tr transport.Transport, devicePath string) error
	Serialize() (string, error)
	// Inflate restores internal state from Serialize() result
	Inflate(serialized string) error
}

type StorageProperties struct {
	InitialEmpty     bool  `json:"initial_empty"`
	RMWSize          int64 `json:"rmw_size"`
	OptimalWriteSize int64 `json:"optimal_size"`
	AtomicSize       int64 `json:"atomic_size"`
}

func GetFromType(fsType string) FilesystemHelper {
	switch fsType {
	case "ext4":
		return NewExt4()
	case "xfs":
		return NewXfs()
	}
	return nil
}

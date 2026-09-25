package volumeinit

import (
	"context"

	"meoe.io/poly-csi/pkg/transport"
)

const PreferredPrefix = "init."

type FilesystemHelper interface {
	Capabilities() Capabilities
	Configure(params map[string]string) error
	SetVolumeProps(props StorageProperties, journalProps StorageProperties) error
	CalcPreferredJournalSize(dataSize int64) int64
	SetSingleDevice(devicePath string)
	SetWithJournal(mainDevice string, journalDevice string)
	Format(ctx context.Context, tr transport.Transport) error
	GetMountOptions() []string
	Resize(ctx context.Context, tr transport.Transport) error
	Serialize() (string, error)
	// Inflate restores internal state from Serialize() result
	Inflate(serialized string) error
}

type Capabilities struct {
	MinFsSize       int64
	ExternalJournal bool
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

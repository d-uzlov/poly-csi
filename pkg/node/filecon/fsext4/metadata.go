package fsext4

import (
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/fsapi"
	"meoe.io/poly-csi/pkg/node/filecon"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type metadata struct {
	User     fsapi.Ext4UserParams `json:"user"`
	Format   fsapi.Ext4Format     `json:"format"`
	MainSize int64                `json:"mainSize"`
}

const prefix = "ext4"

func NewMetadata() filecon.Metadata {
	return &metadata{}
}

func (s *metadata) Configure(requestedSize int64, params map[string]string) error {
	s.MainSize = requestedSize

	s.User = fsapi.Ext4UserParams{
		BytesPerInode:    "65536",
		JournalSizeDenom: 70,
		SeparateJournal:  false,
		ReservedPercent:  1,
	}

	err := serialmaps.Parse(params, prefix, &s.User, serialmaps.WithRemove(true))
	if err != nil {
		return err
	}

	if s.User.JournalSizeDenom < 30 {
		return fmt.Errorf("JournalSizeDenom is too small, min 30")
	}
	if s.User.JournalSizeDenom > 300 {
		return fmt.Errorf("JournalSizeDenom is too big, max 300")
	}

	s.Format.BlockSize = 4096

	return nil
}

const mainVolumeKey = "main"
const journalVolumeKey = "journal"

func (s *metadata) RequestBlockDevices() map[string]filecon.BlockDeviceRequirement {
	result := map[string]filecon.BlockDeviceRequirement{
		mainVolumeKey: {MinSize: s.MainSize},
	}
	if s.User.SeparateJournal {
		result[journalVolumeKey] = filecon.BlockDeviceRequirement{MinSize: s.calcPreferredJournalSize(s.MainSize)}
	}
	return result
}

func (s *metadata) SetVolumeProps(props map[string]filecon.StorageProperties) error {
	if s.User.SeparateJournal {
		journal := props[journalVolumeKey]
		wantSize := s.calcPreferredJournalSize(s.MainSize)
		if journal.Size < wantSize {
			return fmt.Errorf("journal is too small: %d < %d", journal.Size, wantSize)
		}
	}

	main := props[mainVolumeKey]
	s.Format.SkipDiscard = main.InitialEmpty
	return nil
}

func (s *metadata) GetData() fsapi.Data {
	return &fsapi.Ext4Data{
		User:   s.User,
		Format: s.Format,
	}
}

func (s *metadata) GetSize() int64 {
	return s.MainSize
}

func (s *metadata) calcPreferredJournalSize(dataSize int64) int64 {
	bs := s.Format.BlockSize
	// 1024 block, plus superblock
	min := int64(1024*bs + bs)
	// 1 GiB
	max := int64(1024 * 1024 * 1024)

	size := int64(float64(dataSize) / s.User.JournalSizeDenom)
	if size < min {
		return min
	}
	if size > max {
		return max
	}
	return ((size + bs - 1) / bs) * bs
}

func (s *metadata) Serialize() (json.RawMessage, error) {
	return json.Marshal(s)
}

func (s *metadata) Restore(serialized json.RawMessage) error {
	return json.Unmarshal([]byte(serialized), s)
}

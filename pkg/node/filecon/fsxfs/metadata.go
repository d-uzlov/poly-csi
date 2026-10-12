package fsxfs

import (
	"fmt"

	"meoe.io/poly-csi/pkg/api/fsapi"
	"meoe.io/poly-csi/pkg/node/filecon"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type metadata struct {
	User     fsapi.XfsUserParams
	Format   fsapi.XfsFormat
	mainSize int64
}

const prefix = "xfs"

func NewMetadata() filecon.Metadata {
	return &metadata{}
}

func (s *metadata) Configure(requestedSize int64, params map[string]string) error {
	s.mainSize = requestedSize
	const minFsSize = 1024 * 1024 * 1024
	if s.mainSize < minFsSize {
		return fmt.Errorf("xfs size can not smaller than 1GiB")
	}
	err := serialmaps.Parse(params, prefix, &s.User, serialmaps.WithRemove(true))
	if err != nil {
		return err
	}

	s.Format.BlockSize = 4096
	return nil
}

const mainVolumeKey = "main"
const journalVolumeKey = "journal"

func (s *metadata) RequestBlockDevices() map[string]filecon.BlockDeviceRequirement {
	result := map[string]filecon.BlockDeviceRequirement{
		mainVolumeKey: {MinSize: s.mainSize},
	}
	if s.User.SeparateJournal {
		result[journalVolumeKey] = filecon.BlockDeviceRequirement{MinSize: s.calcPreferredJournalSize(s.mainSize)}
	}
	return result
}

func (s *metadata) SetVolumeProps(props map[string]filecon.StorageProperties) error {
	if s.User.SeparateJournal {
		journal := props[journalVolumeKey]
		wantSize := s.calcPreferredJournalSize(s.mainSize)
		if journal.Size < wantSize {
			return fmt.Errorf("journal is too small: %d < %d", journal.Size, wantSize)
		}

		s.Format.JournalStripeUnitBytes = journal.RMWSize
	}

	main := props[mainVolumeKey]
	s.Format.SkipDiscard = main.InitialEmpty

	s.Format.SectorSize = 512
	if main.AtomicSize != 0 {
		if main.AtomicSize > int64(s.Format.BlockSize) {
			s.Format.SectorSize = s.Format.BlockSize
		} else {
			s.Format.SectorSize = main.AtomicSize
		}
	}

	s.Format.SkipDiscard = main.InitialEmpty

	s.Format.StripeUnitBytes = s.Format.BlockSize
	s.Format.StripeWidth = 1
	if main.RMWSize != 0 {
		s.Format.StripeUnitBytes = main.RMWSize
		if main.OptimalWriteSize >= main.RMWSize {
			s.Format.StripeWidth = main.OptimalWriteSize / main.RMWSize
		}
	}

	return nil
}

func (s *metadata) GetData() fsapi.Data {
	return &fsapi.XfsData{
		User:   s.User,
		Format: s.Format,
	}
}

func (s *metadata) calcPreferredJournalSize(dataSize int64) int64 {
	// 64 MiB
	min := int64(64 * 1024 * 1024)
	// 1 GiB
	max := int64(1024 * 1024 * 1024)

	size := int64(float64(dataSize) / s.User.JournalSizeDenom)
	if size < min {
		return min
	}
	if size > max {
		return max
	}
	bs := s.Format.BlockSize
	return ((size + bs - 1) / bs) * bs
}

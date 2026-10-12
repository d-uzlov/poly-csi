package fsapi

const Xfs = Name("xfs")

type XfsUserParams struct {
	SeparateJournal  bool    `map:"separateJournal" json:"separateJournal"`
	BytesPerInode    string  `map:"bytesPerInode" json:"bytes_per_inode"`
	JournalSizeDenom float64 `map:"journalSizeDenom" json:"journal_size_denom"`
	ReservedPercent  string  `map:"reservedPercent" json:"reservedPercent"`
}

type XfsFormat struct {
	BlockSize              int64 `json:"blockSize" map:"blockSize"`
	SectorSize             int64 `json:"sectorSize" map:"sectorSize"`
	SkipDiscard            bool  `json:"skipDiscard" map:"skipDiscard"`
	StripeUnitBytes        int64 `json:"stripeUnitBytes" map:"stripeUnitBytes"`
	StripeWidth            int64 `json:"stripeWidth" map:"stripeWidth"`
	JournalStripeUnitBytes int64 `json:"journalStripeUnitBytes" map:"journalStripeUnitBytes"`
}

type XfsData struct {
	// public
	User   XfsUserParams `map:"user" json:"userParams"`
	Format XfsFormat     `map:"fmt" json:"format"`

	// secret
}

var _ Data = XfsData{}

func (v XfsData) Validate() error {
	return nil
}

func (v XfsData) RegisterSerialmapsType() string {
	return string(Xfs)
}

func (v XfsData) Clone() Data {
	return v
}

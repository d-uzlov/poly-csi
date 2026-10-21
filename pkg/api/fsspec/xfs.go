package fsspec

const Xfs = Name("xfs")

type XfsUserParams struct {
	SeparateJournal  bool    `map:"separateJournal,omitempty" json:"separateJournal,omitempty"`
	BytesPerInode    string  `map:"bytesPerInode,omitempty" json:"bytes_per_inode,omitempty"`
	JournalSizeDenom float64 `map:"journalSizeDenom,omitempty" json:"journal_size_denom,omitempty"`
	ReservedPercent  string  `map:"reservedPercent,omitempty" json:"reservedPercent,omitempty"`
}

type XfsFormat struct {
	BlockSize              int64 `json:"blockSize,omitempty" map:"blockSize,omitempty"`
	SectorSize             int64 `json:"sectorSize,omitempty" map:"sectorSize,omitempty"`
	SkipDiscard            bool  `json:"skipDiscard,omitempty" map:"skipDiscard,omitempty"`
	StripeUnitBytes        int64 `json:"stripeUnitBytes,omitempty" map:"stripeUnitBytes,omitempty"`
	StripeWidth            int64 `json:"stripeWidth,omitempty" map:"stripeWidth,omitempty"`
	JournalStripeUnitBytes int64 `json:"journalStripeUnitBytes,omitempty" map:"journalStripeUnitBytes,omitempty"`
}

type XfsData struct {
	// public
	User   XfsUserParams `map:"user,omitempty" json:"userParams,omitempty"`
	Format XfsFormat     `map:"fmt,omitempty" json:"format,omitempty"`

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

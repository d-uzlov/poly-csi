package fsapi

const Ext4 = Name("ext4")

type Ext4UserParams struct {
	SeparateJournal  bool    `map:"separateJournal" json:"separateJournal"`
	BytesPerInode    string  `map:"bytesPerInode" json:"bytesPerInode"`
	JournalSizeDenom float64 `map:"journalSizeDenom" json:"journalSizeDenom"`
	ReservedPercent  float64 `map:"reservedPercent" json:"reservedPercent"`
}

type Ext4Format struct {
	SkipDiscard bool  `map:"skipDiscard" json:"skipDiscard"`
	BlockSize   int64 `map:"blockSize" json:"blockSize"`
}

type Ext4Data struct {
	// public
	User   Ext4UserParams `map:"user" json:"user"`
	Format Ext4Format     `map:"fmt" json:"format"`

	// secret
}

var _ Data = Ext4Data{}

func (v Ext4Data) Validate() error {
	return nil
}

func (v Ext4Data) RegisterSerialmapsType() string {
	return string(Ext4)
}

func (v Ext4Data) Clone() Data {
	return v
}

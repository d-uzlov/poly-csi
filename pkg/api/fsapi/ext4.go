package fsapi

const Ext4 = Name("ext4")

type Ext4UserParams struct {
	SeparateJournal  bool    `map:"separateJournal,omitempty" json:"separateJournal,omitempty"`
	BytesPerInode    string  `map:"bytesPerInode,omitempty" json:"bytesPerInode,omitempty"`
	JournalSizeDenom float64 `map:"journalSizeDenom,omitempty" json:"journalSizeDenom,omitempty"`
	ReservedPercent  float64 `map:"reservedPercent,omitempty" json:"reservedPercent,omitempty"`
}

type Ext4Format struct {
	SkipDiscard bool  `map:"skipDiscard,omitempty" json:"skipDiscard,omitempty"`
	BlockSize   int64 `map:"blockSize,omitempty" json:"blockSize,omitempty"`
}

type Ext4Data struct {
	// public
	User   Ext4UserParams `map:"user,omitempty" json:"user,omitempty"`
	Format Ext4Format     `map:"fmt,omitempty" json:"format,omitempty"`

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

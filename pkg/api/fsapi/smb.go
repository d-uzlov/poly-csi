package fsapi

const Smb = Name("smb")

type SmbShare struct {
	// public
	ShareName     string `map:"cifs.shareName,omitempty" json:"shareName,omitempty" validate:"required"`
	UserName      string `map:"cifs.userName,omitempty"  json:"userName,omitempty"  validate:"required"`
	ServerAddress string `map:"cifs.server,omitempty"    json:"server,omitempty"    validate:"required"`

	// secret
	UserPassword string `secret:"+" map:"userPassword,omitempty" json:"userPassword,omitempty" validate:"required"`
}

var _ Data = SmbShare{}

func (v SmbShare) Validate() error {
	return nil
}

func (v SmbShare) RegisterSerialmapsType() string {
	return string(Smb)
}

func (v SmbShare) Clone() Data {
	return v
}

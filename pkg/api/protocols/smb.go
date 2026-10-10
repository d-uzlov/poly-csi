package protocols

const Smb = Name("smb")

type SmbShare struct {
	// public
	ShareName string `map:"cifs.shareName,omitempty" json:"shareName,omitempty" validate:"required"`
	UserName  string `map:"cifs.userName,omitempty"  json:"userName,omitempty"  validate:"required"`

	// secret
	UserPassword string `secret:"+" map:"userPassword,omitempty" json:"userPassword,omitempty" validate:"required"`
}

var _ Config = SmbShare{}

func (v SmbShare) Validate() error {
	return nil
}

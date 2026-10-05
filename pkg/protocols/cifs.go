package protocols

import ()

type CifsShare struct {
	// public
	ShareName string `map:"cifs.shareName,omitempty" json:"shareName,omitempty" validate:"required"`
	UserName  string `map:"cifs.userName,omitempty"  json:"userName,omitempty"  validate:"required"`

	// secret
	UserPassword string `secret:"+" map:"userPassword,omitempty" json:"userPassword,omitempty" validate:"required"`
}

func (v CifsShare) Validate() error {
	return nil
}

package protocols

import (
	"fmt"
)

type CifsShare struct {
	// public
	ShareName string `map:"cifs.shareName,omitempty" json:"shareName,omitempty"`
	UserName  string `map:"cifs.userName,omitempty" json:"userName,omitempty"`

	// secret
	UserPassword string `secret:"+" map:"userPassword" json:"userPassword,omitempty"`
}

func (v CifsShare) Validate() error {
	if v.ShareName == "" {
		return fmt.Errorf("missing ShareName")
	}
	if v.UserName == "" {
		return fmt.Errorf("missing UserName")
	}
	if v.UserPassword == "" {
		return fmt.Errorf("missing UserPassword")
	}
	return nil
}

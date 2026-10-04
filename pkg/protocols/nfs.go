package protocols

import ()

type NfsShare struct {
	// public
	Path string `map:"nfs.sharePath,omitempty" json:"path,omitempty" validate:"required,filepath"`

	// secret
}

func (v NfsShare) Validate() error {
	return nil
}

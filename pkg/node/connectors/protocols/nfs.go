package protocols

import (
	"fmt"
)

type Nfs struct {
	// public
	Path string `map:"nfs.sharePath,omitempty" json:"path,omitempty"`

	// secret
}

func (v Nfs) Validate() error {
	if v.Path == "" {
		return fmt.Errorf("missing share path")
	}
	return nil
}

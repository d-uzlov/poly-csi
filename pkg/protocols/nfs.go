package protocols

import (
	"fmt"
)

type NfsShare struct {
	// public
	Path string `map:"nfs.sharePath,omitempty" json:"path,omitempty"`

	// secret
}

func (v NfsShare) Validate() error {
	if v.Path == "" {
		return fmt.Errorf("missing share path")
	}
	return nil
}

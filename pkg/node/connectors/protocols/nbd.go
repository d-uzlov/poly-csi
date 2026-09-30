package protocols

import (
	"fmt"
)

type NbdShare struct {
	// public
	Port       string `map:"nbd.port,omitempty"        json:"port,omitempty"`
	ExportName string `map:"nbd.exportName,omitempty" json:"exportName,omitempty"`

	// secret
}

func (v NbdShare) Validate() error {
	if v.Port == "" {
		return fmt.Errorf("missing port")
	}
	if v.ExportName == "" {
		return fmt.Errorf("missing exportName")
	}
	return nil
}

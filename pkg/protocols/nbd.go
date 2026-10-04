package protocols

import ()

type NbdShare struct {
	// public
	Port       int    `map:"nbd.port,omitempty"       json:"port,omitempty"       validate:"required,port"`
	ExportName string `map:"nbd.exportName,omitempty" json:"exportName,omitempty" validate:"required"`

	// secret
}

func (v NbdShare) Validate() error {
	return nil
}

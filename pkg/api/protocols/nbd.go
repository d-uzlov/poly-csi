package protocols

const Nbd = Name("nbd")

type NbdShare struct {
	// public
	Port       int    `map:"nbd.port,omitempty"       json:"port,omitempty"       validate:"required,port"`
	ExportName string `map:"nbd.exportName,omitempty" json:"exportName,omitempty" validate:"required"`

	// secret
}

var _ Config = NbdShare{}

func (v NbdShare) Validate() error {
	return nil
}

func (v NbdShare) RegisterSerialmapsType() string {
	return string(Nbd)
}

func (v NbdShare) Clone() Config {
	return v
}

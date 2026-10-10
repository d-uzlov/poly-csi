package protocols

const Nfs = Name("nfs")

type NfsShare struct {
	// public
	Path string `map:"nfs.sharePath,omitempty" json:"path,omitempty" validate:"required,filepath"`

	// secret
}

var _ Config = NfsShare{}

func (v NfsShare) Validate() error {
	return nil
}

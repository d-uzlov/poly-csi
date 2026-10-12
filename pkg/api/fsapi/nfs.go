package fsapi

const Nfs = Name("nfs")

type NfsShare struct {
	// public
	Path          string `map:"nfs.sharePath,omitempty" json:"path,omitempty" validate:"required,filepath"`
	ServerAddress string `map:"nfs.server,omitempty"    json:"server,omitempty"    validate:"required"`

	// secret
}

var _ Data = NfsShare{}

func (v NfsShare) Validate() error {
	return nil
}

func (v NfsShare) RegisterSerialmapsType() string {
	return string(Nfs)
}

func (v NfsShare) Clone() Data {
	return v
}

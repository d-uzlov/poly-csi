package protocols

type AoeShare struct {
	// public
	Shelf int `map:"aoe.shelf,omitempty" json:"shelf,omitempty"`
	Slot  int `map:"aoe.slot,omitempty"  json:"slot,omitempty"`

	// secret
}

func (v AoeShare) Validate() error {
	return nil
}

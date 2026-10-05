package protocols

const Aoe = Name("aoe")

type AoeShare struct {
	// public
	Shelf *int `map:"aoe.shelf,omitempty" json:"shelf,omitempty" validate:"required"`
	Slot  *int `map:"aoe.slot,omitempty"  json:"slot,omitempty"  validate:"required,max=255"`

	// secret
}

func (v AoeShare) Validate() error {
	return nil
}

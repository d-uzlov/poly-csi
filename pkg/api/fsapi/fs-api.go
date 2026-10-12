package fsapi

type Data interface {
	Validate() error
	Clone() Data
}

type Name string

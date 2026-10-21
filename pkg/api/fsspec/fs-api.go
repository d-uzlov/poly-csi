package fsspec

type Data interface {
	Validate() error
	Clone() Data
}

type Name string

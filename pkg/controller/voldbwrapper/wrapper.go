package voldbwrapper

import (
	"context"

	"meoe.io/poly-csi/pkg/controller/voldb"
)

type Wrapper struct {
	entry voldb.Entry
}

func New(entry voldb.Entry) Wrapper {
	return Wrapper{entry: entry}
}

func (w Wrapper) Delete(ctx context.Context) error {
	return w.entry.DeleteEntry(ctx)
}

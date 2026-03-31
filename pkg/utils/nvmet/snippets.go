package nvmet

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type Snippets struct {
	// TODO move context to methods?
	ctx     context.Context
	tr      transport.Transport
	useSudo bool
}

func NewSnippets(ctx context.Context, tr transport.Transport, useSudo bool) Snippets {
	return Snippets{
		ctx:     ctx,
		tr:      tr,
		useSudo: useSudo,
	}
}

func (s Snippets) WriteAttribute(parentPath string, attribute string, value string) error {
	if value == "" {
		// kernel configfs treats empty writes as no-op, no point doing them
		return nil
	}
	attrPath := fmt.Sprintf("%s/%s", parentPath, attribute)
	err := s.tr.SudoCmds().Write(s.ctx, attrPath, []byte(value))
	if err != nil {
		return fmt.Errorf("write '%s' to %s: %w", value, attrPath, err)
	}
	return nil
}

func (s Snippets) WriteAttributes(parentPath string, attributes map[string]string) error {
	for k, v := range attributes {
		err := s.WriteAttribute(parentPath, k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

package strbtrfs

import (
	"fmt"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type params struct {
	ParentPath    string `map:"parentPath"`
	CustomOptions string `map:"customOptions"`
}

func parseParams(parser *serialmaps.Parser) (*params, error) {
	var result params
	err := parser.Parse(&result)
	if err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	err = result.validate()
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (p params) validate() error {
	if p.ParentPath == "" {
		return fmt.Errorf("parentPath is empty")
	}
	return nil
}

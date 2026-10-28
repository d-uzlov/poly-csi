package strlvm

import (
	"fmt"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type params struct {
	VgName string `map:"vgName"`
}

func parseParams(parser *serialmaps.Parser) (*params, error) {
	var result params
	err := parser.Parse(&result)

	// no defaults needed

	err = result.validate()
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (p params) validate() error {
	if p.VgName == "" {
		return fmt.Errorf("vgName is empty")
	}
	return nil
}

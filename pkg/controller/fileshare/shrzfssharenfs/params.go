package shrzfssharenfs

import (
	"fmt"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type params struct {
	ShareOptions string `map:"shareOptions"`
}

func parseParams(parser *serialmaps.Parser) (params, error) {
	var result params
	err := parser.Parse(&result)
	if err != nil {
		return params{}, fmt.Errorf("parse params: %w", err)
	}

	if result.ShareOptions == "" {
		return params{}, fmt.Errorf("shareOptions is not set")
	}

	return result, nil
}

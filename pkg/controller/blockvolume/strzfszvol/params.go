package strzfszvol

import (
	"fmt"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type params struct {
	ParentDataset      string `map:"parentDataset"`
	VolBlockSize       string `map:"volblocksize"`
	ReservationPercent string `map:"reservedPercent"`
}

func parseParams(parser *serialmaps.Parser) (*params, error) {

	var result params
	err := parser.Parse(&result)
	if err != nil {
		return nil, fmt.Errorf("static params: %w", err)
	}

	if result.ParentDataset == "" {
		return nil, fmt.Errorf("parentDataset is empty")
	}
	if result.VolBlockSize == "" {
		result.VolBlockSize = "16k"
	}
	if result.ReservationPercent == "" {
		result.ReservationPercent = "100%"
	}

	return &result, nil
}

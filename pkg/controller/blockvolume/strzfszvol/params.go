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

const paramsPrefix = "zfs"

func parseParams(static map[string]string, mutable map[string]string) (*params, error) {

	var result params
	err := serialmaps.Parse(static, paramsPrefix, &result, serialmaps.WithRemove(true))
	if err != nil {
		return nil, fmt.Errorf("static params: %w", err)
	}
	var mutableParams params
	err = serialmaps.Parse(mutable, paramsPrefix, &mutableParams, serialmaps.WithRemove(true))
	if err != nil {
		return nil, fmt.Errorf("mutable params: %w", err)
	}

	if result.ParentDataset == "" {
		return nil, fmt.Errorf("parentDataset is empty")
	}
	if mutableParams.ParentDataset != "" {
		return nil, fmt.Errorf("parentDataset is not supported in mutable params")
	}
	if result.VolBlockSize == "" {
		result.VolBlockSize = "16k"
	}
	if mutableParams.VolBlockSize != "" {
		return nil, fmt.Errorf("VolBlockSize is not supported in mutable params")
	}
	if result.ReservationPercent == "" {
		result.ReservationPercent = "100%"
	}
	if mutableParams.ReservationPercent != "" {
		result.ReservationPercent = mutableParams.ReservationPercent
	}

	return &result, nil
}

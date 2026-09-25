package nodedispatcher

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/multipath"
)

func ensureMultipathDisabled(ctx context.Context, nvi *NodeVolumeInfo, mp multipath.Multipath) error {
	if nvi.MultipathData == "" {
		return nil
	}
	if mp == nil {
		return fmt.Errorf("multipath instance is nil")
	}
	logs.FromCtx(ctx).MicroOp("multipath.Destroy: request")
	err := mp.Destroy(ctx, nvi.MultipathData)
	if err != nil {
		return err
	}
	logs.FromCtx(ctx).MicroOp("multipath.Destroy: success")

	return nil
}

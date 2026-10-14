package strlvm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/blockvolume"
	"meoe.io/poly-csi/pkg/utils/lvm"
)

type Volume struct {
	VgName string `json:"vg_name"`
	LvName string `json:"lv_name"`
	snip   lvm.Snippets
}

var _ blockvolume.Volume = &Volume{}

func newVolume(vgName string, lvName string, snip lvm.Snippets) *Volume {
	return &Volume{
		VgName: vgName,
		LvName: lvName,
		snip:   snip,
	}
}

func parseVolume(serialized string, snip lvm.Snippets) (*Volume, error) {
	var result Volume
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	if result.VgName == "" {
		return nil, fmt.Errorf("VgName is empty")
	}
	if result.LvName == "" {
		return nil, fmt.Errorf("LvName is empty")
	}
	result.snip = snip

	return &result, nil
}

func (v *Volume) Serialize() (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (v *Volume) CreateState(ctx context.Context, size int64) error {
	_, err := v.snip.LvCreate(ctx, v.VgName, v.LvName, size)
	if err != nil {
		return fmt.Errorf("create lv: %w", err)
	}

	return nil
}

func (v *Volume) ReadState(ctx context.Context) (*blockvolume.VolumeState, error) {
	lvs, err := v.snip.ListLvs(ctx, v.VgName, v.LvName)
	if err != nil {
		return nil, fmt.Errorf("checking existing lv: %w", err)
	}
	if len(lvs) == 0 {
		return nil, nil
	}
	lv := lvs[0]
	if lv.LvName != v.LvName {
		return nil, fmt.Errorf("lv %s/%s not found", v.VgName, v.LvName)
	}
	result := blockvolume.VolumeState{
		FsPath: v.snip.MakeLvPath(v.VgName, v.LvName),
		Size:   lv.GetSize(),
	}

	return &result, nil
}

func (v *Volume) DeleteState(ctx context.Context) error {
	_, err := v.snip.LvDelete(ctx, v.VgName, v.LvName)
	if err != nil {
		return err
	}
	return nil
}

func (v *Volume) SetSize(ctx context.Context, newSize int64) error {
	return v.snip.LvSetSize(ctx, v.VgName, v.LvName, newSize)
}

func (v *Volume) MakeSnapshot(ctx context.Context, name string) (blockvolume.Snapshot, error) {
	snapshotName := v.LvName + "." + name

	state, err := v.ReadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if state == nil {
		return nil, fmt.Errorf("read state: volume not found")
	}

	_, err = v.snip.LvSnapshot(ctx, v.VgName, v.LvName, snapshotName, state.Size)
	if err != nil {
		return nil, fmt.Errorf("create lv snapshot '%s': %w", snapshotName, err)
	}

	// err = v.snip.LvSetActive(ctx, v.VgName, snapshotName, false)
	// if err != nil {
	// 	return nil, fmt.Errorf("deactivate lv snapshot '%s': %w", snapshotName, err)
	// }

	return newSnapshot(v.VgName, snapshotName, v.snip), nil
}

func (v *Volume) ReceiveSnapshot(ctx context.Context, sourceFile string, timeout time.Duration) error {
	return fmt.Errorf("unimplemented")
}

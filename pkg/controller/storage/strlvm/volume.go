package strlvm

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/utils/lvm"
)

type Volume struct {
	VgName string `json:"vg_name"`
	LvName string `json:"lv_name"`
	snip   lvm.Snippets
}

var _ storage.Volume = &Volume{}

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

func (v *Volume) GetLocalPath(ctx context.Context) (string, error) {
	return v.snip.MakeLvPath(v.VgName, v.LvName), nil
}

func (v *Volume) Delete(ctx context.Context) error {
	_, err := v.snip.LvDelete(ctx, v.VgName, v.LvName)
	if err != nil {
		return err
	}
	return nil
}

func (v *Volume) GetSize(ctx context.Context) (int64, error) {
	lvs, err := v.snip.ListLvs(ctx, v.VgName, v.LvName)
	if err != nil {
		return 0, fmt.Errorf("checking existing lv: %w", err)
	}
	if len(lvs) == 0 {
		return 0, fmt.Errorf("lv %s/%s not found", v.VgName, v.LvName)
	}
	lv := lvs[0]
	if lv.LvName != v.LvName {
		return 0, fmt.Errorf("lv %s/%s not found", v.VgName, v.LvName)
	}
	return lv.GetSize(), nil
}

func (v *Volume) Expand(ctx context.Context, newSize int64) (int64, error) {
	currentSize, err := v.GetSize(ctx)
	if err != nil {
		return 0, err
	}
	if newSize <= currentSize {
		return currentSize, nil
	}

	err = v.snip.LvSetSize(ctx, v.VgName, v.LvName, newSize)
	if err != nil {
		return 0, err
	}

	return newSize, nil
}

func (v *Volume) MakeSnapshot(ctx context.Context, name string) (storage.Snapshot, error) {
	snapshotName := v.LvName + "." + name

	size, err := v.GetSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("get lv size '%s': %w", snapshotName, err)
	}

	_, err = v.snip.LvSnapshot(ctx, v.VgName, v.LvName, snapshotName, size)
	if err != nil {
		return nil, fmt.Errorf("create lv snapshot '%s': %w", snapshotName, err)
	}

	// err = v.snip.LvSetActive(ctx, v.VgName, snapshotName, false)
	// if err != nil {
	// 	return nil, fmt.Errorf("deactivate lv snapshot '%s': %w", snapshotName, err)
	// }

	return newSnapshot(v.VgName, snapshotName, v.snip), nil
}

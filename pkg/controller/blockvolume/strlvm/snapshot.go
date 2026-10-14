package strlvm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/blockvolume"
	"meoe.io/poly-csi/pkg/utils/lvm"
)

type Snapshot struct {
	VgName string `json:"vg_name"`
	LvName string `json:"lv_name"`
	snip   lvm.Snippets
}

var _ blockvolume.Snapshot = &Snapshot{}

func newSnapshot(vgName string, lvName string, snip lvm.Snippets) *Snapshot {
	return &Snapshot{
		VgName: vgName,
		LvName: lvName,
		snip:   snip,
	}
}

func parseSnapshot(serialized json.RawMessage, snip lvm.Snippets) (*Snapshot, error) {
	var result Snapshot
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

func (s *Snapshot) Serialize() (json.RawMessage, error) {
	return json.Marshal(s)
}

func (s *Snapshot) GetLocalPath(ctx context.Context) (path string, release func(), err error) {
	return s.snip.MakeLvPath(s.VgName, s.LvName), func() {}, nil
}

func (s *Snapshot) Send(ctx context.Context, targetFile string, timeout time.Duration) error {
	return fmt.Errorf("unimplemented")
}

func (s *Snapshot) Delete(ctx context.Context) error {
	_, err := s.snip.LvDelete(ctx, s.VgName, s.LvName)
	return err
}

package shrtniscsi

import (
	"context"
	"encoding/json"
	"fmt"
	"uuid"

	"meoe.io/poly-csi/pkg/controller/blockshare"
	"meoe.io/poly-csi/pkg/controller/blockvolume/strzfszvol"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport `json:"-"`
	gp globalParams        `json:"-"`

	Iqn  string `json:"iqn"`
	Zvol string `json:"zvol"`
}

var _ blockshare.Share = &share{}

func parseShare(data string) (*share, error) {
	var result share
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (sh *share) Serialize() (string, error) {
	encoded, err := json.Marshal(sh)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (sh *share) Validate() error {
	if sh.tr == nil {
		return fmt.Errorf("transport is nil")
	}
	if sh.gp.iqnBase == "" {
		return fmt.Errorf("missing iqn base")
	}
	if sh.Iqn == "" {
		return fmt.Errorf("missing iqn")
	}
	if sh.Zvol == "" {
		return fmt.Errorf("missing zvol")
	}
	return nil
}

func (sh *share) PrepareShare(ctx context.Context) error {
	// TODO split configuration
	return nil
}

func (sh *share) AttachPath(ctx context.Context, fsPath string, index int) error {
	return fmt.Errorf("unimplemented")
}

func (sh *share) AttachVolume(ctx context.Context, volume any, index int) error {
	zfsVol, ok := volume.(*strzfszvol.Volume)
	if !ok {
		return fmt.Errorf("storage volume is not zfs zvol: %T", volume)
	}
	sh.Zvol = zfsVol.Path

	snip := newSnippets(sh.tr)

	err := snip.setupTarget(ctx, tnIscsiRequest{
		extentName: uuid.NewV7().String(),
		iqn:        sh.Iqn,
		zvol:       zfsVol.Path,
	})
	if err != nil {
		return err
	}

	return nil
}

func (sh *share) DeleteShare(ctx context.Context) error {
	snip := newSnippets(sh.tr)

	err := snip.removeIscsiTarget(ctx, sh.Iqn)
	if err != nil {
		return err
	}
	err = snip.removeIscsiExtents(ctx, sh.Zvol)
	if err != nil {
		return err
	}

	return nil
}

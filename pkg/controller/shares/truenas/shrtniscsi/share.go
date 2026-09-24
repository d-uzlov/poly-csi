package shrtniscsi

import (
	"context"
	"encoding/json"
	"fmt"
	"uuid"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/controller/storage/strzfszvol"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport `json:"-"`
	gp globalParams        `json:"-"`

	Iqn  string `json:"iqn"`
	Zvol string `json:"zvol"`
}

var _ shares.Share = &share{}

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

func (sh *share) SharePath(ctx context.Context, fsPath string) error {
	return fmt.Errorf("unimplemented")
}

func (sh *share) ShareVolume(ctx context.Context, volume any) error {
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

package shrtniscsi

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport
	gp globalParams

	Iqn  string
	Zvol string
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

func (sh *share) MakeShare(ctx context.Context, fsPath string) error {
	return fmt.Errorf("unimplemented")

	// zfsVol, ok := req.Volume.(*strzfszvol.Volume)
	// if !ok {
	// 	return nil, fmt.Errorf("storage volume is not zfs zvol: %T", req.Volume)
	// }

	// iqn := fmt.Sprintf("%s:%s", m.gp.iqnBase, req.ShareId)

	// snip := newSnippets(m.tr)

	// err = snip.setupTarget(ctx, tnIscsiRequest{
	// 	extentName: uuid.New().String(),
	// 	iqn:        iqn,
	// 	zvol:       zfsVol.Path,
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return nil
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

package shrnvmetkeeper

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mntrnvme"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/nvmet"
)

type publishData struct {
	HostNqn string `json:"host_nqn"`
}

func (d *publishData) Serialize() (string, error) {
	err := d.validate()
	if err != nil {
		return "", err
	}

	encoded, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (d publishData) validate() error {
	if d.HostNqn == "" {
		return fmt.Errorf("missing host_nqn")
	}
	return nil
}

func parsePublishData(serialized string) (publishData, error) {
	var result publishData
	err := json.Unmarshal([]byte(serialized), &result)
	if err != nil {
		return result, err
	}
	err = result.validate()
	if err != nil {
		return result, err
	}
	return result, nil
}

func (sh *share) Publish(ctx context.Context, shareData string, mi mounts.MounterNodeInfo) (*shares.PublishContext, error) {
	c, err := parseShareData(shareData)
	if err != nil {
		return nil, fmt.Errorf("parse share data: %w", err)
	}

	nvmeInfo, err := mntrnvme.ParseNvmeInfo(mi)
	if err != nil {
		return nil, fmt.Errorf("parse mounter info as nvme: %w", err)
	}

	cmd := append(sh.cmds.nvmetKeeperCmd, "host", "create", nvmet.Host{
		Nqn:      nvmeInfo.HostNqn,
		NeedAuth: false,
	}.Json())
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("host create: %w", err)
	}

	cmd = append(sh.cmds.nvmetKeeperCmd, "subsys", "allow", c.Nqn, nvmeInfo.HostNqn)
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("subsys allow: %w", err)
	}

	pc := map[string]string{}
	err = mntrnvme.InjectPublishContext(pc, mntrnvme.NvmePublishContext{
		Nqn:       c.Nqn,
		Port:      c.Port,
		Transport: c.Transport,
		IoQueues:  c.IoQueues,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateVolumeContext: %w", err)
	}

	pd := &publishData{
		HostNqn: nvmeInfo.HostNqn,
	}

	return &shares.PublishContext{
		Public: pc,
		Data:   pd,
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, shareData string, serialized string) error {
	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}
	pd, err := parsePublishData(serialized)
	if err != nil {
		return fmt.Errorf("parse publish data: %w", err)
	}

	cmd := append(sh.cmds.nvmetKeeperCmd, "subsys", "disallow", c.Nqn, pd.HostNqn)
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("subsys disallow: %w", err)
	}

	return nil
}

func (sh *share) UnpublishAll(ctx context.Context, shareData string) error {
	return fmt.Errorf("UnpublishAll is not implemented")
}

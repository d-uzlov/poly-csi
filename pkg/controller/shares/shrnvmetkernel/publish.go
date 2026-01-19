package shrnvmetkernel

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mntrnvme"
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
	if !sh.cmds.ready(ctx) {
		return nil, fmt.Errorf("nvmet is not ready")
	}
	c, err := parseShareData(shareData)
	if err != nil {
		return nil, fmt.Errorf("parse share data: %w", err)
	}

	snip := nvmet.NewSnippets(ctx, true)

	nvmeInfo, err := mntrnvme.ParseNvmeInfo(mi)
	if err != nil {
		return nil, fmt.Errorf("parse mounter info as nvme: %w", err)
	}
	err = snip.ConfigureHost(nvmet.Host{
		Nqn: nvmeInfo.HostNqn,
	})
	if err != nil {
		return nil, fmt.Errorf("configure host: %w", err)
	}
	err = snip.AllowHost(c.Nqn, nvmeInfo.HostNqn)
	if err != nil {
		return nil, fmt.Errorf("allow host: %w", err)
	}

	err = sh.cmds.save(ctx)
	if err != nil {
		return nil, fmt.Errorf("nvmet_save: %w", err)
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
	if !sh.cmds.ready(ctx) {
		return fmt.Errorf("nvmet is not ready")
	}
	c, err := parseShareData(shareData)
	if err != nil {
		return fmt.Errorf("parse share data: %w", err)
	}
	pd, err := parsePublishData(serialized)
	if err != nil {
		return fmt.Errorf("parse publish data: %w", err)
	}

	snip := nvmet.NewSnippets(ctx, true)

	err = snip.DisallowHost(c.Nqn, pd.HostNqn)
	if err != nil {
		return fmt.Errorf("disallow host: %w", err)
	}

	err = sh.cmds.save(ctx)
	if err != nil {
		return fmt.Errorf("nvmet_save: %w", err)
	}

	return nil
}

func (sh *share) UnpublishAll(ctx context.Context, shareData string) error {
	return fmt.Errorf("UnpublishAll is not implemented")
}

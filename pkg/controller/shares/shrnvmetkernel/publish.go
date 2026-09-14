package shrnvmetkernel

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrnvme"
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

func (sh *share) Publish(ctx context.Context, mi connectors.ConnectorNodeInfo) (*shares.PublishContext, error) {
	if !testReady(ctx, sh.tr) {
		return nil, fmt.Errorf("nvmet is not ready")
	}

	snip := nvmet.NewSnippets(ctx, sh.tr, true)

	nvmeInfo, err := cntrnvme.ParseNvmeInfo(mi)
	if err != nil {
		return nil, fmt.Errorf("parse connector info as nvme: %w", err)
	}
	err = snip.ConfigureHost(nvmet.Host{
		Nqn: nvmeInfo.HostNqn,
	})
	if err != nil {
		return nil, fmt.Errorf("configure host: %w", err)
	}
	err = snip.AllowHost(sh.Nqn, nvmeInfo.HostNqn)
	if err != nil {
		return nil, fmt.Errorf("allow host: %w", err)
	}

	err = saveState(ctx, sh.tr)
	if err != nil {
		return nil, fmt.Errorf("nvmet_save: %w", err)
	}

	return &shares.PublishContext{
		Public: cntrnvme.PublicContext{
			TargetNqn:     sh.Nqn,
			HostNqn:       nvmeInfo.HostNqn,
			HostId:        nvmeInfo.HostId,
			AuthType:      cntrnvme.AuthTypeNqn,
			ConnectPort:   sh.Port,
			TransportType: sh.Transport,
		},
		LocalData: &publishData{
			HostNqn: nvmeInfo.HostNqn,
		},
	}, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized string) error {
	if !testReady(ctx, sh.tr) {
		return fmt.Errorf("nvmet is not ready")
	}
	pd, err := parsePublishData(serialized)
	if err != nil {
		return fmt.Errorf("parse publish data: %w", err)
	}

	snip := nvmet.NewSnippets(ctx, sh.tr, true)

	err = snip.DisallowHost(sh.Nqn, pd.HostNqn)
	if err != nil {
		return fmt.Errorf("disallow host: %w", err)
	}

	err = saveState(ctx, sh.tr)
	if err != nil {
		return fmt.Errorf("nvmet_save: %w", err)
	}

	return nil
}

func (sh *share) UnpublishAll(ctx context.Context) error {
	return fmt.Errorf("UnpublishAll is not implemented")
}

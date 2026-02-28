package shrnvmetkeeper

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/node/connectors/cntrnvme"
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

func (sh *share) Publish(ctx context.Context, shareData string, mi connectors.ConnectorNodeInfo) (*shares.PublishContext, error) {
	c, err := parseShareData(shareData)
	if err != nil {
		return nil, fmt.Errorf("parse share data: %w", err)
	}

	nvmeInfo, err := cntrnvme.ParseNvmeInfo(mi)
	if err != nil {
		return nil, fmt.Errorf("parse connector info as nvme: %w", err)
	}

	// todo this will create ephemeral one-use hosts on every publish
	// we need to try to limit this and run cleanup when possible
	hostId := uuid.NewString()
	hostNqn := nvmeInfo.HostNqn + ":" + hostId

	secret := map[string]string{}
	authType := ""

	switch c.AuthType {
	case authTypeNone:
		// no need to configure hosts
	case authTypeNqn:
		authType = cntrnvme.AuthTypeNqn

		cmd := append(sh.cmds.nvmetKeeperCmd, "host", "create", nvmet.Host{
			Nqn:      hostNqn,
			NeedAuth: false,
		}.Json())
		err = transport.CheckedRun(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("host create: %w", err)
		}
	case authTypeChapProvision:
		authType = cntrnvme.AuthTypeDhChap

		cmd := []string{"sudo", "nvme", "gen-dhchap-key", "--key-length=32", "--hmac=0"}
		hostKey, err := transport.CheckedRunStdout(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("host create: %w", err)
		}
		if hostKey == "" {
			return nil, fmt.Errorf("failed to generate host dh-chap key via nvme gen-dhchap-key: empty output")
		}
		cmd = []string{"sudo", "nvme", "gen-dhchap-key", "--key-length=32", "--hmac=0"}
		controllerKey, err := transport.CheckedRunStdout(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("host create: %w", err)
		}
		if controllerKey == "" {
			return nil, fmt.Errorf("failed to generate controller dh-chap key via nvme gen-dhchap-key: empty output")
		}

		err = cntrnvme.NvmeSecrets{
			HostKey:       hostKey,
			ControllerKey: controllerKey,
		}.Inject(secret)
		if err != nil {
			return nil, fmt.Errorf("inject nvme secret publish context: %w", err)
		}

		cmd = append(sh.cmds.nvmetKeeperCmd, "host", "create", nvmet.Host{
			Nqn:           hostNqn,
			NeedAuth:      true,
			HostKey:       hostKey,
			ControllerKey: controllerKey,
			HashType:      nvmet.HashTypeHmacSha256,
			DhGroup:       nvmet.DhGroup2048,
		}.Json())
		err = transport.CheckedRun(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("host create: %w", err)
		}
	}

	cmd := append(sh.cmds.nvmetKeeperCmd, "subsys", "allow", c.Nqn, hostNqn)
	err = transport.CheckedRun(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("subsys allow: %w", err)
	}

	pc := map[string]string{}
	err = cntrnvme.NvmePublishContext{
		TargetNqn:     c.Nqn,
		HostNqn:       hostNqn,
		HostId:        hostId,
		AuthType:      authType,
		ConnectPort:   c.Port,
		TransportType: c.Transport,
		IoQueues:      c.IoQueues,
	}.Inject(pc)
	if err != nil {
		return nil, fmt.Errorf("inject nvme public publish context: %w", err)
	}

	pd := &publishData{
		HostNqn: hostNqn,
	}

	return &shares.PublishContext{
		Public: pc,
		Data:   pd,
		Secret: secret,
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

package shrnvmetkeeper

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/api/protocols"
	"meoe.io/poly-csi/pkg/controller/shares"
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

func (sh *share) Publish(ctx context.Context, req shares.PublishRequest) (*shares.PublishContext, error) {
	hostNqn := sh.Nqn + ":" + req.NodeName
	hostId := uuid.NewSHA1(uuid.Max, []byte(hostNqn)).String()

	pc := protocols.NvmeShare{
		TargetNqn:     sh.Nqn,
		HostNqn:       hostNqn,
		HostId:        hostId,
		ConnectPort:   sh.TcpPort,
		TransportType: sh.Transport,
	}

	switch sh.AuthType {
	case authTypeNone:
		pc.AuthType = ""
	case authTypeNqn:
		pc.AuthType = protocols.NvmeAuthNqn

		cmd := append(nvmetKeeperCmd, "host", "create", nvmet.Host{
			Nqn:      hostNqn,
			NeedAuth: false,
		}.Json())
		err := transport.CheckedRun(ctx, sh.tr, cmd)
		if err != nil {
			return nil, fmt.Errorf("host create: %w", err)
		}
	case authTypeChapProvision:
		pc.AuthType = protocols.NvmeAuthDhChap

		host, err := sh.createSecureHost(ctx, hostNqn)
		if err != nil {
			return nil, fmt.Errorf("createSecureHost: %w", err)
		}
		pc.HostKey = host.HostKey
		pc.ControllerKey = host.ControllerKey
	}

	cmd := append(nvmetKeeperCmd, "subsys", "allow", sh.Nqn, hostNqn)
	err := transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("subsys allow: %w", err)
	}

	return &shares.PublishContext{
		Protocol: pc,
		LocalData: &publishData{
			HostNqn: hostNqn,
		},
	}, nil
}

func (sh *share) createSecureHost(ctx context.Context, hostNqn string) (*nvmet.Host, error) {
	cmd := []string{"nvme", "gen-dhchap-key", "--key-length=32", "--hmac=0"}
	hostKey, err := transport.CheckedRunStdout(ctx, sh.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("host create: %w", err)
	}
	hostKey = strings.TrimSpace(hostKey)
	if hostKey == "" {
		return nil, fmt.Errorf("failed to generate host dh-chap key via nvme gen-dhchap-key: empty output")
	}

	cmd = []string{"nvme", "gen-dhchap-key", "--key-length=32", "--hmac=0"}
	controllerKey, err := transport.CheckedRunStdout(ctx, sh.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("host create: %w", err)
	}
	controllerKey = strings.TrimSpace(controllerKey)
	if controllerKey == "" {
		return nil, fmt.Errorf("failed to generate controller dh-chap key via nvme gen-dhchap-key: empty output")
	}

	hostInfo := nvmet.Host{
		Nqn:           hostNqn,
		NeedAuth:      true,
		HostKey:       hostKey,
		ControllerKey: controllerKey,
		HashType:      nvmet.HashTypeHmacSha256,
		DhGroup:       nvmet.DhGroup2048,
	}
	cmd = append(nvmetKeeperCmd, "host", "create", hostInfo.Json())
	err = transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("host create: %w", err)
	}

	return &hostInfo, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized string) error {
	pd, err := parsePublishData(serialized)
	if err != nil {
		return fmt.Errorf("parse publish data: %w", err)
	}

	cmd := append(nvmetKeeperCmd, "subsys", "disallow", sh.Nqn, pd.HostNqn)
	err = transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return fmt.Errorf("subsys disallow: %w", err)
	}

	return nil
}

func (sh *share) UnpublishAll(ctx context.Context) error {
	return fmt.Errorf("UnpublishAll is not implemented")
}

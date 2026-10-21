package shrnvmetkeeper

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/controller/blockshare"
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

func parsePublishData(serialized json.RawMessage) (publishData, error) {
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

func (sh *share) Publish(ctx context.Context, req blockshare.PublishRequest) (*blockshare.PublishContext, error) {
	hostNqn := sh.Nqn + ":" + req.NodeName
	hostId := uuid.NewSHA1(uuid.Max, []byte(hostNqn)).String()

	pc := blockspec.NvmeShare{
		TargetNqn:     sh.Nqn,
		HostNqn:       hostNqn,
		HostId:        hostId,
		ConnectPort:   sh.TcpPort,
		TransportType: sh.Transport,
		ServerAddress: req.AddressList,
	}

	switch sh.AuthType {
	case authTypeNone:
		pc.AuthType = ""
	case authTypeNqn:
		pc.AuthType = blockspec.NvmeAuthNqn

		cmd := append(nvmetKeeperCmd, "host", "create", nvmet.Host{
			Nqn:      hostNqn,
			NeedAuth: false,
		}.Json())
		err := transport.CheckedRun(ctx, sh.tr, cmd)
		if err != nil {
			return nil, fmt.Errorf("host create: %w", err)
		}
	case authTypeChapProvision:
		pc.AuthType = blockspec.NvmeAuthDhChap

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

	return &blockshare.PublishContext{
		Protocol: pc,
		LocalData: &publishData{
			HostNqn: hostNqn,
		},
	}, nil
}

func (sh *share) generateKey(source string) string {
	if sh.Secret == nil {
		panic("sh.Secret == nil")
	}
	h := hmac.New(sha256.New, sh.Secret)
	h.Write([]byte(source))
	rawBytes := h.Sum(nil)

	checksum := crc32.ChecksumIEEE(rawBytes)
	checksumBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(checksumBytes, checksum)

	b64 := base64.RawStdEncoding.EncodeToString(append(rawBytes, checksumBytes...))
	return fmt.Sprintf("DHHC-1:01:%s:", b64)
}

func (sh *share) createSecureHost(ctx context.Context, hostNqn string) (*nvmet.Host, error) {
	hostKey := sh.generateKey(hostNqn + ":host_key")
	controllerKey := sh.generateKey(hostNqn + ":controller_key")

	hostInfo := nvmet.Host{
		Nqn:           hostNqn,
		NeedAuth:      true,
		HostKey:       hostKey,
		ControllerKey: controllerKey,
		HashType:      nvmet.HashTypeHmacSha256,
		DhGroup:       nvmet.DhGroup2048,
	}
	cmd := append(nvmetKeeperCmd, "host", "create", hostInfo.Json())
	err := transport.CheckedRun(ctx, sh.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("host create: %w", err)
	}

	return &hostInfo, nil
}

func (sh *share) Unpublish(ctx context.Context, serialized json.RawMessage) error {
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

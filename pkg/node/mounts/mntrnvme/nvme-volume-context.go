package mntrnvme

import (
	"fmt"
)

type NvmePublishContext struct {
	TargetNqn     string
	HostNqn       string
	HostId        string
	AuthType      string
	ConnectPort   string
	TransportType string
	IoQueues      string
}

type NvmeSecrets struct {
	HostKey       string
	ControllerKey string
}

const (
	AuthTypeNqn    = "nqn"
	AuthTypeDhChap = "dhchap"
)

const (
	keyTargetNqn = "nvme.target_nqn"

	keyHostNqn = "nvme.host_nqn"

	keyHostId = "nvme.host_id"

	keyAuthType = "nvme.auth_type"

	keyConnectPort = "nvme.connect_port"

	keyTransportType = "nvme.transport_type"

	keyIoQueues = "nvme.io_queues"

	keyHostKey = "nvme.hostKey"

	keyControllerKey = "nvme.controllerKey"
)

func (c NvmePublishContext) Validate() error {
	if c.TargetNqn == "" {
		return fmt.Errorf("target nqn is empty")
	}
	if c.HostNqn == "" {
		return fmt.Errorf("host nqn is empty")
	}
	if c.HostId == "" {
		return fmt.Errorf("host id is empty")
	}
	switch c.AuthType {
	case AuthTypeNqn, AuthTypeDhChap:
		// ok
	default:
		return fmt.Errorf("unknown auth type: %s", c.AuthType)
	}
	if c.ConnectPort == "" {
		return fmt.Errorf("connect port is empty")
	}
	if c.TransportType == "" {
		return fmt.Errorf("transport type is empty")
	}
	// it's ok if IoQueues is missing
	return nil
}

func (c NvmePublishContext) Inject(dst map[string]string) error {
	err := c.Validate()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	dst[keyTargetNqn] = c.TargetNqn
	dst[keyHostNqn] = c.HostNqn
	dst[keyHostId] = c.HostId
	dst[keyAuthType] = c.AuthType
	dst[keyConnectPort] = c.ConnectPort
	dst[keyTransportType] = c.TransportType
	dst[keyIoQueues] = c.IoQueues
	return nil
}

func ParsePublishContext(src map[string]string) (NvmePublishContext, error) {
	targetNqn := src[keyTargetNqn]
	hostNqn := src[keyHostNqn]
	hostId := src[keyHostId]
	authType := src[keyAuthType]
	port := src[keyConnectPort]
	transport := src[keyTransportType]
	ioQueues := src[keyIoQueues]

	result := NvmePublishContext{
		TargetNqn:     targetNqn,
		HostNqn:       hostNqn,
		HostId:        hostId,
		AuthType:      authType,
		ConnectPort:   port,
		TransportType: transport,
		IoQueues:      ioQueues,
	}
	err := result.Validate()
	if err != nil {
		return NvmePublishContext{}, fmt.Errorf("validate: %w", err)
	}
	return result, nil
}

func (c NvmeSecrets) Validate() error {
	if c.HostKey == "" {
		return fmt.Errorf("host key is empty")
	}
	if c.ControllerKey == "" {
		return fmt.Errorf("controller key is empty")
	}
	return nil
}

func (c NvmeSecrets) Inject(dst map[string]string) error {
	err := c.Validate()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	dst[keyHostKey] = c.HostKey
	dst[keyControllerKey] = c.ControllerKey
	return nil
}

func ParseSecretContext(src map[string]string) (NvmeSecrets, error) {
	hostKey := src[keyHostKey]
	controllerKey := src[keyControllerKey]

	result := NvmeSecrets{
		HostKey:       hostKey,
		ControllerKey: controllerKey,
	}

	err := result.Validate()
	if err != nil {
		return NvmeSecrets{}, fmt.Errorf("validate: %w", err)
	}

	return result, nil
}

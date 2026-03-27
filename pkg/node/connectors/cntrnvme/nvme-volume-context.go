package cntrnvme

import (
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type PublicContext struct {
	TargetNqn     string `map:"target_nqn"`
	HostNqn       string `map:"host_nqn"`
	HostId        string `map:"host_id"`
	AuthType      string `map:"auth_type"`
	ConnectPort   string `map:"connect_port"`
	TransportType string `map:"transport_type"`
	IoQueues      string `map:"io_queues"`
}

type SecretContext struct {
	HostKey       string `map:"host_key"`
	ControllerKey string `map:"controller_key"`
}

const (
	AuthTypeNqn    = "nqn"
	AuthTypeDhChap = "dhchap"
)

func validateContext(public PublicContext, secret SecretContext) error {
	if public.TargetNqn == "" {
		return fmt.Errorf("target nqn is empty")
	}
	if public.HostNqn == "" {
		return fmt.Errorf("host nqn is empty")
	}
	if public.HostId == "" {
		return fmt.Errorf("host id is empty")
	}
	switch public.AuthType {
	case AuthTypeNqn, AuthTypeDhChap:
		// ok
	default:
		return fmt.Errorf("unknown auth type: %s", public.AuthType)
	}
	if public.ConnectPort == "" {
		return fmt.Errorf("connect port is empty")
	}
	if public.TransportType == "" {
		return fmt.Errorf("transport type is empty")
	}
	// it's ok if IoQueues is missing
	switch public.AuthType {
	case "":
		return fmt.Errorf("missing auth type")
	case AuthTypeNqn:
		// nothing to do here
	case AuthTypeDhChap:
		if secret.HostKey == "" {
			return fmt.Errorf("missing host key")
		}
		if secret.ControllerKey == "" {
			return fmt.Errorf("missing controller key")
		}
	default:
		return fmt.Errorf("auth type: unknown value: '%s'", public.AuthType)
	}
	return nil
}

func ParsePublishContext(public map[string]string, secret []byte) (PublicContext, SecretContext, error) {
	var resPublic PublicContext
	err := serialmaps.Parse(public, "", &resPublic)
	if err != nil {
		return PublicContext{}, SecretContext{}, fmt.Errorf("public: %w", err)
	}

	var resSecret SecretContext
	if len(secret) != 0 {
		err = json.Unmarshal(secret, &resSecret)
		if err != nil {
			return PublicContext{}, SecretContext{}, fmt.Errorf("secret: %w", err)
		}
	}

	return resPublic, resSecret, nil
}

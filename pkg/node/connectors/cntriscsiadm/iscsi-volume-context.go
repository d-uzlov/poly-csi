package cntriscsiadm

import (
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

const (
	AuthNone                  = "none"
	AuthChapFromPublishSecret = "publish-secret-mutual"
)

type PublicContext struct {
	Iqn       string `map:"iscsi.nqn"`
	Port      string `map:"iscsi.port"`
	Transport string `map:"iscsi.transport"`
	AuthType  string `map:"iscsi.chap"`
}

type SecretContext struct {
	ClientUser string `map:"iscsi.client_user"`
	ClientPass string `map:"iscsi.client_pass"`
	TargetUser string `map:"iscsi.target_user"`
	TargetPass string `map:"iscsi.target_pass"`
}

func validateContext(public PublicContext, secret SecretContext) error {
	if public.Iqn == "" {
		return fmt.Errorf("missing iqn")
	}
	if public.Port == "" {
		return fmt.Errorf("missing port")
	}
	if public.Transport == "" {
		return fmt.Errorf("missing transport")
	}
	switch public.AuthType {
	case "":
		return fmt.Errorf("missing auth type")
	case AuthNone:
		// nothing to do here
	case AuthChapFromPublishSecret:
		if secret.ClientUser == "" {
			return fmt.Errorf("missing client user")
		}
		if secret.ClientPass == "" {
			return fmt.Errorf("missing client pass")
		}
		if secret.TargetUser == "" {
			return fmt.Errorf("missing target user")
		}
		if secret.TargetPass == "" {
			return fmt.Errorf("missing target pass")
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
	err = json.Unmarshal(secret, &resSecret)
	if err != nil {
		return PublicContext{}, SecretContext{}, fmt.Errorf("secret: %w", err)
	}

	return resPublic, resSecret, nil
}

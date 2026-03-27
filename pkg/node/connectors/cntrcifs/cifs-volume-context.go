package cntrcifs

import (
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type PublicContext struct {
	ShareName string `map:"cifs.share_name"`
	UserName  string `map:"cifs.user_name"`
}

type SecretContext struct {
	UserPassword string `json:"cifs.user_pass"`
}

const (
	shareNameKey = "cifs.share_name"
	userNameKey  = "cifs.user_name"
	userPassKey  = "cifs.user_pass"
)

func validateContext(public PublicContext, secret SecretContext) error {
	if public.ShareName == "" {
		return fmt.Errorf("missing field %s", shareNameKey)
	}
	if public.UserName == "" {
		return fmt.Errorf("missing field %s", userNameKey)
	}
	if secret.UserPassword == "" {
		return fmt.Errorf("missing field %s", userPassKey)
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

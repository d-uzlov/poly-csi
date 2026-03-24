package cntrnfs

import (
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type PublicContext struct {
	Path string `map:"nfs.share_path"`
}

type SecretContext struct {
}

func validateContext(public PublicContext, secret SecretContext) error {
	if public.Path == "" {
		return fmt.Errorf("missing share path")
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

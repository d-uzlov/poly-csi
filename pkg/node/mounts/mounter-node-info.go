package mounts

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/encrypt"
)

type MounterNodeInfo struct {
	Public string `json:"public"`
	Secret string `json:"secret"`
}

func (m MounterNodeInfo) Serialize(aesSecret []byte) (string, error) {
	secret := ""
	if m.Secret != "" {
		if aesSecret == nil {
			return "", fmt.Errorf("mounter has secret values but aesSecret is missing")
		}
		ciphertext, err := encrypt.EncryptAes(aesSecret, m.Secret)
		if err != nil {
			return "", fmt.Errorf("EncryptAes: %w", err)
		}
		secret = base64.StdEncoding.EncodeToString(ciphertext)
	}
	encoded, err := json.Marshal(MounterNodeInfo{
		Public: m.Public,
		Secret: secret,
	})
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func DeserializeMounterNodeInfo(raw string, aesSecret []byte) (MounterNodeInfo, error) {
	var result MounterNodeInfo
	err := json.Unmarshal([]byte(raw), &result)
	if err != nil {
		return MounterNodeInfo{}, err
	}

	if aesSecret == nil || result.Secret == "" {
		return MounterNodeInfo{
			Public: result.Public,
			Secret: "",
		}, nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(result.Secret)
	if err != nil {
		return MounterNodeInfo{}, fmt.Errorf("parse base64: %w", err)
	}
	plaintext, err := encrypt.DecryptAes(aesSecret, ciphertext)
	if err != nil {
		return MounterNodeInfo{}, fmt.Errorf("decrypt: %w", err)
	}
	result.Secret = string(plaintext)
	return result, nil
}

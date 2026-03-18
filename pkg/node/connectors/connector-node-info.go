package connectors

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/encrypt"
)

type ConnectorNodeInfo struct {
	Public string `json:"public"`
	Secret string `json:"secret"`
}

func (m ConnectorNodeInfo) Serialize(aesSecret []byte) (string, error) {
	secret := ""
	if m.Secret != "" {
		if aesSecret == nil {
			return "", fmt.Errorf("connector has secret values but aesSecret is missing")
		}
		ciphertext, err := encrypt.EncryptAes(aesSecret, m.Secret)
		if err != nil {
			return "", fmt.Errorf("EncryptAes: %w", err)
		}
		secret = base64.StdEncoding.EncodeToString(ciphertext)
	}
	encoded, err := json.Marshal(ConnectorNodeInfo{
		Public: m.Public,
		Secret: secret,
	})
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func DeserializeConnectorNodeInfo(raw string, aesSecret []byte) (ConnectorNodeInfo, error) {
	var result ConnectorNodeInfo
	err := json.Unmarshal([]byte(raw), &result)
	if err != nil {
		return ConnectorNodeInfo{}, err
	}

	if aesSecret == nil || result.Secret == "" {
		return ConnectorNodeInfo{
			Public: result.Public,
			Secret: "",
		}, nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(result.Secret)
	if err != nil {
		return ConnectorNodeInfo{}, fmt.Errorf("parse base64: %w", err)
	}
	plaintext, err := encrypt.DecryptAes(aesSecret, ciphertext)
	if err != nil {
		return ConnectorNodeInfo{}, fmt.Errorf("decrypt: %w", err)
	}
	result.Secret = string(plaintext)
	return result, nil
}

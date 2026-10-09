package encrypt

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

const secretPrefix = "_secret"
const encTypeAesGcm = "aes_gcm"

type SecretWrapper struct {
	PubKeyHash      string `map:"pub_key_hash"`
	EncType         string `map:"enc_type"`
	AesKeyPkBase64  string `map:"aes_key_pk_base64"`
	SecretAesBase64 string `map:"secret_aes_base64"`
}

func ExtractSecretFromMap(src map[string]string, privateKey RsaPrivateKey) ([]byte, error) {
	var wrap SecretWrapper
	err := serialmaps.ParseRemove(src, secretPrefix, true, &wrap)
	if err != nil {
		return nil, fmt.Errorf("parse secret: %w", err)
	}
	if wrap.SecretAesBase64 == "" {
		return nil, nil
	}

	if wrap.PubKeyHash == "" {
		return nil, fmt.Errorf("missing public key hash")
	}
	if wrap.EncType == "" {
		return nil, fmt.Errorf("missing enc type")
	}
	if wrap.AesKeyPkBase64 == "" {
		return nil, fmt.Errorf("missing secret key")
	}

	if wrap.PubKeyHash != privateKey.GetPublicKey().Hash() {
		return nil, fmt.Errorf("public key hash does not match private key")
	}

	if wrap.EncType != encTypeAesGcm {
		return nil, fmt.Errorf("unknown enc type: '%s'", wrap.EncType)
	}

	aesKeyPk, err := base64.StdEncoding.DecodeString(wrap.AesKeyPkBase64)
	if err != nil {
		return nil, fmt.Errorf("aes key: decode base64: %w", err)
	}
	aesKey, err := privateKey.Decrypt(aesKeyPk)
	if err != nil {
		return nil, fmt.Errorf("eas key: decrypt pk: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(wrap.SecretAesBase64)
	if err != nil {
		return nil, fmt.Errorf("ciphertext: decode base64: %w", err)
	}
	pcSecretPlaintext, err := DecryptAes(aesKey, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("ciphertext: DecryptAes: %w", err)
	}

	return pcSecretPlaintext, nil
}

func InjectSecretIntoMap(dst map[string]string, publicKey RsaPublicKey, src any) error {
	aesKey := make([]byte, 32)
	_, err := rand.Read(aesKey)
	if err != nil {
		return fmt.Errorf("generate aes key: %w", err)
	}
	aesKeyPk, err := publicKey.Encrypt(aesKey)
	if err != nil {
		return fmt.Errorf("aes key: encrypt pk: %w", err)
	}
	secretJson, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("marshal secret: %w", err)
	}
	secretJsonEnc, err := EncryptAes(aesKey, secretJson)
	if err != nil {
		return fmt.Errorf("EncryptAes: %w", err)
	}

	wrap := SecretWrapper{
		PubKeyHash:      publicKey.Hash(),
		EncType:         encTypeAesGcm,
		AesKeyPkBase64:  base64.StdEncoding.EncodeToString(aesKeyPk),
		SecretAesBase64: base64.StdEncoding.EncodeToString(secretJsonEnc),
	}
	_, err = serialmaps.SerializeInto(wrap, dst, secretPrefix)
	if err != nil {
		return fmt.Errorf("serialize: %w", err)
	}
	return nil
}

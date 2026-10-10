package encrypt

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const encTypeAesGcm = "aes_gcm"

type SecretWrapper struct {
	aesKey []byte `map:"-"`
	// PubKeyHash is the hash of the public key used to create this secret.
	// Should be checked to avoid decoding errors when there is a key mismatch.
	PubKeyHash string `map:"pub_key_hash" validate:"required"`
	// EncType describes how secret data was encrypted
	EncType string `map:"enc_type" validate:"required,oneof=aes_gcm"`
	// EncryptedAesKey is base64(rsa(aes_key))
	EncryptedAesKey string `map:"aes_key" validate:"required"`
	// Ciphertext is base64(aes(data))
	Ciphertext string `map:"ciphertext" validate:"required"`
}

func NewWrapper(publicKey RsaPublicKey) (SecretWrapper, error) {
	result := SecretWrapper{
		PubKeyHash: publicKey.Hash(),
		aesKey:     make([]byte, 32),
		EncType:    encTypeAesGcm,
	}
	// rand.Read is guaranteed to not fail
	_, _ = rand.Read(result.aesKey)
	aesKeyPk, err := publicKey.Encrypt(result.aesKey)
	if err != nil {
		return SecretWrapper{}, fmt.Errorf("encrypt aes key: %w", err)
	}
	result.EncryptedAesKey = base64.StdEncoding.EncodeToString(aesKeyPk)

	return result, nil
}

func (wrap *SecretWrapper) SetData(data []byte) error {
	ciphertext, err := EncryptAes(wrap.aesKey, data)
	if err != nil {
		return fmt.Errorf("EncryptAes: %w", err)
	}
	wrap.Ciphertext = base64.StdEncoding.EncodeToString(ciphertext)
	return nil
}

func (wrap *SecretWrapper) Extract(privateKey RsaPrivateKey) ([]byte, error) {
	if wrap.Ciphertext == "" {
		return nil, nil
	}

	privKeyHash := privateKey.GetPublicKey().Hash()
	if wrap.PubKeyHash != privKeyHash {
		return nil, fmt.Errorf("public key hash '%s' does not match private key hash '%s'", wrap.PubKeyHash, privKeyHash)
	}

	encryptedAesKey, err := base64.StdEncoding.DecodeString(wrap.EncryptedAesKey)
	if err != nil {
		return nil, fmt.Errorf("aes key: decode base64: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(wrap.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("ciphertext: decode base64: %w", err)
	}

	switch wrap.EncType {
	case encTypeAesGcm:
		aesKey, err := privateKey.Decrypt(encryptedAesKey)
		if err != nil {
			return nil, fmt.Errorf("eas key: decrypt pk: %w", err)
		}

		pcSecretPlaintext, err := DecryptAes(aesKey, ciphertext)
		if err != nil {
			return nil, fmt.Errorf("ciphertext: DecryptAes: %w", err)
		}

		return pcSecretPlaintext, nil
	default:
		return nil, fmt.Errorf("unknown encryption type '%s'", wrap.EncType)
	}
}

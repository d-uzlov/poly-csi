package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

func EncryptAes(aesSecret []byte, data string) ([]byte, error) {
	aes, err := aes.NewCipher(aesSecret)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	_, err = rand.Read(nonce)
	if err != nil {
		return nil, err
	}
	// store nonce in result
	ciphertext := gcm.Seal(nonce, nonce, []byte(data), nil)
	return ciphertext, nil
}

func DecryptAes(aesSecret []byte, ciphertext []byte) ([]byte, error) {
	aes, err := aes.NewCipher(aesSecret)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return nil, err
	}

	// we know that beginning of result contains nonce
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, []byte(nonce), []byte(ciphertext), nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func EncryptMapByKeySuffix(src map[string]string, suffix string, aesSecret []byte) (map[string]string, error) {
	result := map[string]string{}
	for k, v := range src {
		if !strings.HasSuffix(k, suffix) {
			result[k] = v
			continue
		}
		ciphertext, err := EncryptAes(aesSecret, v)
		if err != nil {
			return nil, fmt.Errorf("'%s': encrypt: %w", k, err)
		}
		result[k] = base64.StdEncoding.EncodeToString(ciphertext)
	}
	return result, nil
}

func DecryptMapByKeySuffix(src map[string]string, suffix string, aesSecret []byte) (map[string]string, error) {
	result := map[string]string{}
	for k, v := range src {
		if !strings.HasSuffix(k, suffix) {
			result[k] = v
			continue
		}
		ciphertext, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("'%s': parse base64: %w", k, err)
		}
		plaintext, err := DecryptAes(aesSecret, ciphertext)
		if err != nil {
			return nil, fmt.Errorf("'%s': decrypt: %w", k, err)
		}
		result[k] = string(plaintext)
	}
	return result, nil
}

func InjectAesSecret(src map[string]string, pubKey RsaPublicKey, aesSecret []byte) {
	aesSecretEncrypted, err := pubKey.Encrypt(aesSecret)
	if err != nil {
		panic(err)
	}
	src["_secret_public_key_hash"] = pubKey.Hash()
	src["_secret_enc_type"] = "aes_gcm"
	src["_secret_key"] = base64.StdEncoding.EncodeToString(aesSecretEncrypted)
}

func ExtractAesSecret(src map[string]string, privateKey RsaPrivateKey) ([]byte, error) {
	pubKeyHash := src["_secret_public_key_hash"]
	if pubKeyHash == "" {
		return nil, fmt.Errorf("_secret_public_key_hash: field missing")
	}
	if pubKeyHash != privateKey.GetPublicKey().Hash() {
		return nil, fmt.Errorf("_secret_public_key_hash: value does not match local private key")
	}
	reqEncryptType := src["_secret_enc_type"]
	if reqEncryptType == "" {
		return nil, fmt.Errorf("_secret_enc_type: field missing")
	}
	if reqEncryptType != "aes_gcm" {
		return nil, fmt.Errorf("_secret_enc_type: unknown type '%s'", reqEncryptType)
	}
	reqAesKeyB64 := src["_secret_key"]
	if reqAesKeyB64 == "" {
		return nil, fmt.Errorf("_secret_key: field missing")
	}
	reqAesKeyEncrypted, err := base64.StdEncoding.DecodeString(reqAesKeyB64)
	if err != nil {
		return nil, fmt.Errorf("_secret_key: parsing base64 failed: %w", err)
	}
	reqAesKey, err := privateKey.Decrypt(reqAesKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("_secret_key: decrypt by private key: %w", err)
	}
	return reqAesKey, nil
}

package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

func EncryptAes(key []byte, data []byte) ([]byte, error) {
	aes, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return nil, fmt.Errorf("NewGCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	_, err = rand.Read(nonce)
	if err != nil {
		return nil, err
	}
	// ciphertext will start with unencrypted nonce
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func DecryptAes(key []byte, ciphertext []byte) ([]byte, error) {
	aes, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return nil, fmt.Errorf("NewGCM: %w", err)
	}

	// we know that beginning of result contains nonce
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm.Open: %w", err)
	}

	return plaintext, nil
}

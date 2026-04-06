package encrypt

import (
	"crypto"
	"crypto/dsa"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

const pemTagPrivate = "PRIVATE KEY"
const pemTagPublic = "PUBLIC KEY"

type RsaPrivateKey struct {
	key *rsa.PrivateKey
}

func ParsePrivateRsaPem(privateKeyBytes []byte) (*RsaPrivateKey, error) {
	privateKeyPem, _ := pem.Decode(privateKeyBytes)
	if privateKeyPem == nil {
		return nil, fmt.Errorf("failed to parse private key as PEM")
	}
	if privateKeyPem.Type != pemTagPrivate {
		return nil, fmt.Errorf("unexpected private key type: %s", privateKeyPem.Type)
	}

	privateKeyAny, err := x509.ParsePKCS8PrivateKey(privateKeyPem.Bytes)
	if err != nil {
		return nil, err
	}
	switch key := privateKeyAny.(type) {
	case *rsa.PrivateKey:
		return &RsaPrivateKey{
			key: key,
		}, nil
	case *ecdsa.PrivateKey, ed25519.PrivateKey, *ecdh.PrivateKey:
		return nil, fmt.Errorf("key type not supported: %T", privateKeyAny)
	}
	return nil, fmt.Errorf("unknown private key type: %T", privateKeyAny)
}

func GenerateRsa() (*RsaPrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &RsaPrivateKey{
		key: privateKey,
	}, nil
}

func (e *RsaPrivateKey) SerializePem() []byte {
	bytes, err := x509.MarshalPKCS8PrivateKey(e.key)
	if err != nil {
		panic(err)
	}
	return pem.EncodeToMemory(
		&pem.Block{
			Type:  pemTagPrivate,
			Bytes: bytes,
		},
	)
}

func (e *RsaPrivateKey) GetPublicKey() RsaPublicKey {
	return RsaPublicKey{public: &e.key.PublicKey}
}

func (e *RsaPrivateKey) Encrypt(data []byte) (encrypted []byte, err error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, &e.key.PublicKey, data, nil)
}

func (e *RsaPrivateKey) Decrypt(encrypted []byte) (data []byte, err error) {
	return e.key.Decrypt(nil, encrypted, &rsa.OAEPOptions{Hash: crypto.SHA256})
}

type RsaPublicKey struct {
	public *rsa.PublicKey
}

func ParsePublicRsaPem(keyBytes []byte) (*RsaPublicKey, error) {
	keyPem, _ := pem.Decode(keyBytes)
	if keyPem == nil {
		return nil, fmt.Errorf("failed to parse public key as PEM")
	}
	if keyPem.Type != pemTagPublic {
		return nil, fmt.Errorf("unexpected public key type: %s", keyPem.Type)
	}

	keyAny, err := x509.ParsePKIXPublicKey(keyPem.Bytes)
	if err != nil {
		return nil, fmt.Errorf("ParsePKIXPublicKey: %w", err)
	}
	switch key := keyAny.(type) {
	case *rsa.PublicKey:
		return &RsaPublicKey{
			public: key,
		}, nil
	case *dsa.PublicKey, *ecdsa.PublicKey, ed25519.PublicKey, *ecdh.PublicKey:
		return nil, fmt.Errorf("key type not supported: %T", keyAny)
	}
	return nil, fmt.Errorf("unknown private key type: %T", keyAny)
}

func (e RsaPublicKey) Validate() error {
	if e.public == nil {
		return fmt.Errorf("not initialized")
	}
	return nil
}

func (e RsaPublicKey) Hash() string {
	hash := sha256.Sum256(e.SerializePem())
	return base64.StdEncoding.EncodeToString(hash[:])
}

func (e RsaPublicKey) Encrypt(data []byte) (encrypted []byte, err error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, e.public, data, nil)
}

func (e RsaPublicKey) SerializePem() []byte {
	bytes, err := x509.MarshalPKIXPublicKey(e.public)
	if err != nil {
		panic(err)
	}
	return pem.EncodeToMemory(
		&pem.Block{
			Type:  pemTagPublic,
			Bytes: bytes,
		},
	)
}

func LoadPrivateKey(filepath string) (*RsaPrivateKey, error) {
	keyBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	rsa, err := ParsePrivateRsaPem(keyBytes)
	if err != nil {
		return nil, err
	}
	return rsa, nil
}

func LoadPublicKey(filepath string) (*RsaPublicKey, error) {
	keyBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	rsa, err := ParsePublicRsaPem(keyBytes)
	if err != nil {
		return nil, err
	}
	return rsa, nil
}

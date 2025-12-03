package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
)

var (
	ErrKeysAlreadyExist = errors.New("keys already exist")
)

// Sign signs the given data with the given key.
func Sign(keyData, data string) (string, error) {
	if data == "" {
		return "", fmt.Errorf("data is empty")
	}
	if keyData == "" {
		return "", fmt.Errorf("keyData is nil")
	}
	keyData = strings.Replace(keyData, "\\r\\n", "\r\n", -1)
	bytes, err := privateKeyFromBytes([]byte(keyData))
	if err != nil {
		return "", err
	}

	signature := ed25519.Sign(bytes, []byte(data))

	return base64.StdEncoding.EncodeToString(signature), nil
}

// Verify verifies the given data with the given key.
func Verify(keyData, data, sig string) (bool, error) {
	if keyData == "" {
		return false, fmt.Errorf("keyData is empty")
	}

	keyData = strings.Replace(keyData, "\\r\\n", "\r\n", -1)

	publicKey, err := publicKeyFromBytes([]byte(keyData))
	if err != nil {
		return false, err
	}

	sigData, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return false, err
	}

	return ed25519.Verify(publicKey, []byte(data), sigData), nil
}

// GenerateKeys writes PEM-encoded Ed25519 keys.
func GenerateKeys(privKeyPath, pubKeyPath string) error {
	var (
		err   error
		b     []byte
		block *pem.Block
		pub   ed25519.PublicKey
		priv  ed25519.PrivateKey
	)

	if _, err = os.Stat(privKeyPath); err == nil {
		return ErrKeysAlreadyExist

	}
	if _, err = os.Stat(pubKeyPath); err == nil {
		return ErrKeysAlreadyExist
	}

	pub, priv, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	b, err = x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}

	block = &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: b,
	}

	err = os.WriteFile(privKeyPath, pem.EncodeToMemory(block), 0600)
	if err != nil {
		return err
	}

	b, err = x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return err
	}

	block = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: b,
	}

	err = os.WriteFile(pubKeyPath, pem.EncodeToMemory(block), 0644)
	if err != nil {
		return err
	}

	return nil
}

func SignFile(privateKeyPath, message string) (string, error) {
	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return "", err
	}

	sig := ed25519.Sign(privateKey, []byte(message))
	return base64.StdEncoding.EncodeToString(sig), nil
}

func VerifyFile(pubKeyPath, message, sig string) (bool, error) {
	pub, err := loadPublicKey(pubKeyPath)
	if err != nil {
		return false, err
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return false, err
	}

	return ed25519.Verify(pub, []byte(message), sigBytes), nil
}

// VerifyRaw verifies data using a raw Ed25519 public key (32-byte slice).
// sig is base64-encoded.
func VerifyRaw(pub []byte, data, sig string) (bool, error) {
	if len(pub) == 0 {
		return false, fmt.Errorf("public key is empty")
	}

	sigData, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return false, err
	}

	return ed25519.Verify(ed25519.PublicKey(pub), []byte(data), sigData), nil
}

func PublicKeyFromFile(pubKeyPath string) ([]byte, error) {
	pub, err := loadPublicKey(pubKeyPath)
	if err != nil {
		return nil, err
	}
	return pub, nil
}

func PrivateKeyFromFile(privKeyPath string) ([]byte, error) {
	priv, err := loadPrivateKey(privKeyPath)
	if err != nil {
		return nil, err
	}
	return priv, nil
}

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return privateKeyFromBytes(data)
}

func loadPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return publicKeyFromBytes(data)
}

func publicKeyFromBytes(pubKey []byte) ([]byte, error) {
	block, _ := pem.Decode(pubKey)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("invalid public key PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key.(ed25519.PublicKey), nil
}

func privateKeyFromBytes(privKey []byte) ([]byte, error) {
	block, _ := pem.Decode(privKey)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("invalid private key PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key.(ed25519.PrivateKey), nil
}

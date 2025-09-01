package flowbaker

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// ExecutorAuthHelper provides helper methods for executor authentication
type ExecutorAuthHelper struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	executorID string
}

// NewExecutorAuthHelper creates a new executor authentication helper
func NewExecutorAuthHelper(executorID string, privateKeyBase64 string) (*ExecutorAuthHelper, error) {
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKeyBytes))
	}

	privateKey := ed25519.PrivateKey(privateKeyBytes)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return &ExecutorAuthHelper{
		privateKey: privateKey,
		publicKey:  publicKey,
		executorID: executorID,
	}, nil
}

// GenerateKeyPair generates a new Ed25519 key pair for executor authentication
func GenerateKeyPair() (privateKeyBase64, publicKeyBase64 string, err error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate key pair: %w", err)
	}

	privateKeyBase64 = base64.StdEncoding.EncodeToString(privateKey)
	publicKeyBase64 = base64.StdEncoding.EncodeToString(publicKey)

	return privateKeyBase64, publicKeyBase64, nil
}

// GetPublicKeyBase64 returns the base64-encoded public key
func (h *ExecutorAuthHelper) GetPublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(h.publicKey)
}

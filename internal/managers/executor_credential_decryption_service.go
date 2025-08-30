package managers

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/flowbaker/flowbaker/internal/domain"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// executorCredentialDecryptionService handles credential decryption on the executor side
type executorCredentialDecryptionService struct {
	privateKey string // Base64 encoded X25519 private key
}

func NewExecutorCredentialDecryptionService(privateKeyBase64 string) *executorCredentialDecryptionService {
	return &executorCredentialDecryptionService{
		privateKey: privateKeyBase64,
	}
}

func (s *executorCredentialDecryptionService) DecryptCredential(encryptedCred domain.EncryptedExecutionCredential) ([]byte, error) {
	// Check expiry
	if time.Now().Unix() > encryptedCred.ExpiresAt {
		return nil, errors.New("credential expired")
	}

	// Decode executor's base64 private key
	executorPrivateKey, err := decodeX25519KeySlice(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode executor private key: %w", err)
	}

	// Perform X25519 ECDH to get shared secret
	sharedSecret, err := curve25519.X25519(executorPrivateKey, encryptedCred.EphemeralPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	// Derive encryption key using HKDF
	encryptionKey, err := deriveEncryptionKey(sharedSecret, encryptedCred.ExecutorID)
	if err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	// Decrypt credential with ChaCha20-Poly1305
	payloadJSON, err := decryptChaCha20Poly1305(encryptedCred.EncryptedPayload, encryptionKey, encryptedCred.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credential payload: %w", err)
	}

	return payloadJSON, nil
}

func decodeX25519KeySlice(base64Key string) ([]byte, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 encoding: %w", err)
	}

	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(keyBytes))
	}

	return keyBytes, nil
}

func decodeEd25519Key(base64Key string) (ed25519.PublicKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 encoding: %w", err)
	}

	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid key length: expected %d bytes, got %d", ed25519.PublicKeySize, len(keyBytes))
	}

	return ed25519.PublicKey(keyBytes), nil
}

func decryptChaCha20Poly1305(ciphertext []byte, key []byte, nonce []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create ChaCha20-Poly1305 cipher: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

func deriveEncryptionKey(sharedSecret []byte, executorID string) ([]byte, error) {
	salt := []byte("flowbaker-executor-credentials")
	info := []byte("encryption-key-" + executorID)

	hkdf := hkdf.New(sha256.New, sharedSecret, salt, info)
	key := make([]byte, chacha20poly1305.KeySize) // 32 bytes

	// Read exactly the key size from HKDF
	if _, err := io.ReadFull(hkdf, key); err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	return key, nil
}

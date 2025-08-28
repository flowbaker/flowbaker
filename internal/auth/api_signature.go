package auth

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"
)

// APIRequestSigner signs API requests for executor authentication
type APIRequestSigner struct {
	privateKey ed25519.PrivateKey
}

// NewAPIRequestSigner creates a new request signer
func NewAPIRequestSigner(privateKeyBase64 string) (*APIRequestSigner, error) {
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKeyBytes))
	}

	return &APIRequestSigner{
		privateKey: ed25519.PrivateKey(privateKeyBytes),
	}, nil
}

// SignRequest creates signature headers for an API request
func (s *APIRequestSigner) SignRequest(method, path string, body []byte) (map[string]string, error) {
	timestamp := time.Now().Unix()
	timestampStr := strconv.FormatInt(timestamp, 10)

	// Create body hash
	bodyHash := sha256.Sum256(body)
	bodyHashStr := fmt.Sprintf("sha256:%x", bodyHash)

	// Create canonical request
	canonicalRequest := fmt.Sprintf("%s\n%s\n\n%s\n%s", method, path, timestampStr, bodyHashStr)

	// Sign the canonical request
	signature := ed25519.Sign(s.privateKey, []byte(canonicalRequest))
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	return map[string]string{
		"X-API-Signature": fmt.Sprintf("ed25519=%s", signatureB64),
		"X-API-Timestamp": timestampStr,
	}, nil
}

// APISignatureVerifier verifies API request signatures
type APISignatureVerifier struct {
	publicKey ed25519.PublicKey
}

// NewAPISignatureVerifier creates a new signature verifier
func NewAPISignatureVerifier(publicKeyBase64 string) (*APISignatureVerifier, error) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKeyBytes))
	}

	return &APISignatureVerifier{
		publicKey: ed25519.PublicKey(publicKeyBytes),
	}, nil
}

// VerifyRequest verifies the signature of an API request
func (v *APISignatureVerifier) VerifyRequest(method, path, signatureHeader, timestampHeader string, body []byte) error {
	// Parse signature header
	if len(signatureHeader) < 9 || signatureHeader[:8] != "ed25519=" {
		return fmt.Errorf("invalid signature format")
	}
	signatureB64 := signatureHeader[8:]

	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Parse timestamp
	timestamp, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	// Check timestamp window (5 minutes)
	now := time.Now().Unix()
	timeDiff := now - timestamp
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > 300 { // 5 minutes
		return fmt.Errorf("timestamp outside allowed window")
	}

	// Create body hash
	bodyHash := sha256.Sum256(body)
	bodyHashStr := fmt.Sprintf("sha256:%x", bodyHash)

	// Create canonical request
	canonicalRequest := fmt.Sprintf("%s\n%s\n\n%s\n%s", method, path, timestampHeader, bodyHashStr)

	// Verify signature
	if !ed25519.Verify(v.publicKey, []byte(canonicalRequest), signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

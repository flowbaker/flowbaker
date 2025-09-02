package initialization

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"golang.org/x/crypto/curve25519"
)

func GenerateExecutorName() string {
	adjectives := []string{"swift", "bright", "calm", "bold", "wise", "kind", "quick", "cool", "warm", "clear"}
	nouns := []string{"river", "mountain", "ocean", "forest", "valley", "meadow", "stream", "peak", "lake", "field"}

	now := time.Now()
	adjIndex := int(now.UnixNano()) % len(adjectives)
	nounIndex := int(now.UnixNano()/1000) % len(nouns)

	return fmt.Sprintf("%s_%s", adjectives[adjIndex], nouns[nounIndex])
}

func GenerateX25519KeyPair() (privateKeyBase64, publicKeyBase64 string, err error) {
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	publicKey, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	privateKeyBase64 = base64.StdEncoding.EncodeToString(privateKey[:])
	publicKeyBase64 = base64.StdEncoding.EncodeToString(publicKey)

	return privateKeyBase64, publicKeyBase64, nil
}

func GenerateAllKeys() (CryptoKeys, error) {
	var keys CryptoKeys

	x25519Private, x25519Public, err := GenerateX25519KeyPair()
	if err != nil {
		return keys, fmt.Errorf("failed to generate X25519 keys: %w", err)
	}

	ed25519Private, ed25519Public, err := flowbaker.GenerateKeyPair()
	if err != nil {
		return keys, fmt.Errorf("failed to generate Ed25519 keys: %w", err)
	}

	keys.X25519Private = x25519Private
	keys.X25519Public = x25519Public
	keys.Ed25519Private = ed25519Private
	keys.Ed25519Public = ed25519Public

	return keys, nil
}

func GetDefaultAPIURL() string {
	if strings.Contains(strings.ToLower(strings.Join([]string{}, "")), "dev") {
		return "http://localhost:8080"
	}

	return "https://api.flowbaker.io"
}

func GetVerificationURL(apiURL string) string {
	if strings.Contains(apiURL, "localhost") {
		return "http://localhost:3000"
	}

	return "https://app.flowbaker.io"
}

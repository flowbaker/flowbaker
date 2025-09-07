package initialization

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"golang.org/x/crypto/curve25519"
)

func GenerateExecutorName() string {
	adjectives := []string{
		"swift", "bright", "calm", "bold", "wise", "kind", "quick", "cool", "warm", "clear",
		"gentle", "fierce", "noble", "clever", "silent", "vibrant", "steady", "ancient", "mystic", "radiant",
		"powerful", "graceful", "mysterious", "elegant", "resilient", "dynamic", "serene", "mighty", "luminous", "agile",
		"fearless", "brilliant", "tranquil", "robust", "stellar", "cosmic", "ethereal", "infinite", "blazing", "crystal",
	}
	nouns := []string{
		"river", "mountain", "ocean", "forest", "valley", "meadow", "stream", "peak", "lake", "field",
		"thunder", "lightning", "storm", "breeze", "sunrise", "sunset", "galaxy", "comet", "phoenix", "dragon",
		"wolf", "eagle", "falcon", "tiger", "bear", "lion", "shark", "whale", "dolphin", "hawk",
		"crystal", "diamond", "emerald", "sapphire", "ruby", "pearl", "opal", "quartz", "amber", "jade",
	}

	adjBytes := make([]byte, 4)
	nounBytes := make([]byte, 4)
	rand.Read(adjBytes)
	rand.Read(nounBytes)

	adjIndex := int(adjBytes[0])<<24 | int(adjBytes[1])<<16 | int(adjBytes[2])<<8 | int(adjBytes[3])
	nounIndex := int(nounBytes[0])<<24 | int(nounBytes[1])<<16 | int(nounBytes[2])<<8 | int(nounBytes[3])

	adjIndex = adjIndex % len(adjectives)
	nounIndex = nounIndex % len(nouns)

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
	// Check for development mode indicators
	if isDevMode() {
		return "http://localhost:8080"
	}

	return "https://api.flowbaker.io"
}

func isDevMode() bool {
	// Check common development environment variables
	devEnvVars := []string{
		"FLOWBAKER_DEV",
		"DEVELOPMENT",
		"DEV_MODE",
	}

	for _, envVar := range devEnvVars {
		if value := os.Getenv(envVar); value != "" && value != "false" && value != "0" {
			return true
		}
	}

	// Check if GO_ENV is set to development
	if goEnv := os.Getenv("GO_ENV"); goEnv == "development" || goEnv == "dev" {
		return true
	}

	// Check if NODE_ENV is set to development (common in mixed environments)
	if nodeEnv := os.Getenv("NODE_ENV"); nodeEnv == "development" {
		return true
	}

	return false
}

func GetVerificationURL(apiURL string) string {
	// Use same development mode detection as API URL
	if strings.Contains(apiURL, "localhost") || isDevMode() {
		return "https://localhost:5173"
	}

	return "https://app.flowbaker.io"
}

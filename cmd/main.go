package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"flowbaker/internal/auth"
	"flowbaker/internal/server"
	"flowbaker/pkg/flowbaker"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/curve25519"
)

func main() {
	// Setup logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Parse command line arguments
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "run":
		runExecutor()
	case "generate-keys":
		generateKeys(args[1:])
	default:
		log.Error().Str("command", args[0]).Msg("Unknown command")
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`Usage: %s <command> [options]

Commands:
  run                    Start the executor service
  generate-keys          Generate cryptographic keys for executor

Examples:
  %s run
  %s generate-keys --executor-id my-executor

Configuration:
  The executor can be configured via:
  1. Config file: executor_config.yaml (see executor_config.yaml.example)
  2. Environment variables (override config file values)
  
  Config file locations searched:
  - ./executor_config.yaml
  - ./config/executor_config.yaml  
  - ~/.flowbaker/executor_config.yaml

Required Settings (via config file or environment):
  EXECUTOR_ID                    Unique executor identifier
  API_BASE_URL                   Flowbaker API base URL
  EXECUTOR_X25519_PRIVATE_KEY    Base64 X25519 private key for encryption
  EXECUTOR_ED25519_PRIVATE_KEY   Base64 Ed25519 private key for signing
  
Optional Settings:
  HTTP_ADDRESS                   HTTP server address (default: :8081)

`, os.Args[0], os.Args[0], os.Args[0])
}

// generateX25519KeyPair generates a new X25519 key pair for encryption
func generateX25519KeyPair() (privateKeyBase64, publicKeyBase64 string, err error) {
	// Generate a private key
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Generate the corresponding public key
	publicKey, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	privateKeyBase64 = base64.StdEncoding.EncodeToString(privateKey[:])
	publicKeyBase64 = base64.StdEncoding.EncodeToString(publicKey)

	return privateKeyBase64, publicKeyBase64, nil
}

func generateKeys(args []string) {
	var executorID string

	// Parse arguments for executor ID
	for i, arg := range args {
		if arg == "--executor-id" && i+1 < len(args) {
			executorID = args[i+1]
			break
		}
	}

	if executorID == "" {
		log.Error().Msg("--executor-id is required for key generation")
		fmt.Printf("Usage: %s generate-keys --executor-id <executor-id>\n", os.Args[0])
		os.Exit(1)
	}

	log.Info().Str("executor_id", executorID).Msg("Generating keys for executor")

	// Generate X25519 key pair
	x25519Private, x25519Public, err := generateX25519KeyPair()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to generate X25519 key pair")
	}

	// Generate Ed25519 key pair
	ed25519Private, ed25519Public, err := flowbaker.GenerateKeyPair()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to generate Ed25519 key pair")
	}

	// Output environment variables
	fmt.Printf("\n# Executor Keys for %s\n", executorID)
	fmt.Printf("# Copy these environment variables to your .env file or export them\n\n")
	fmt.Printf("export EXECUTOR_ID=%s\n", executorID)
	fmt.Printf("export EXECUTOR_X25519_PRIVATE_KEY=%s\n", x25519Private)
	fmt.Printf("export EXECUTOR_ED25519_PRIVATE_KEY=%s\n", ed25519Private)
	fmt.Printf("\n# Public keys (for reference only, not needed by executor)\n")
	fmt.Printf("# X25519_PUBLIC_KEY=%s\n", x25519Public)
	fmt.Printf("# ED25519_PUBLIC_KEY=%s\n", ed25519Public)
	fmt.Printf("\n# Next steps:\n")
	fmt.Printf("# 1. Set API_BASE_URL environment variables\n")
	fmt.Printf("# 2. Run: %s run\n", os.Args[0])

	log.Info().Msg("Key generation completed")
}

func runExecutor() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info().Msg("Starting executor service")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	log.Info().
		Str("executor_id", cfg.ExecutorID).
		Str("api_base_url", cfg.APIBaseURL).
		Msg("Executor configuration loaded")

	if cfg.Ed25519PrivateKey == "" {
		log.Fatal().Msg("EXECUTOR_ED25519_PRIVATE_KEY is required")
	}

	flowbakerClient := flowbaker.NewClient(
		flowbaker.WithBaseURL(cfg.APIBaseURL),
		flowbaker.WithExecutorID(cfg.ExecutorID),
		flowbaker.WithEd25519PrivateKey(cfg.Ed25519PrivateKey),
	)

	log.Info().Msg("Flowbaker client with signature-based auth ready")

	deps, err := BuildExecutorDependencies(context.Background(), ExecutorDependencyConfig{
		FlowbakerClient: flowbakerClient,
		ExecutorID:      cfg.ExecutorID,
		Config:          cfg,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to build executor dependencies")
	}

	var apiSignatureVerifier *auth.APISignatureVerifier
	if cfg.APISigningPublicKey != "" {
		var err error
		apiSignatureVerifier, err = auth.NewAPISignatureVerifier(cfg.APISigningPublicKey)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create API signature verifier")
		}
		log.Info().Msg("API signature verification enabled")
	} else {
		log.Warn().Msg("API signature verification disabled - no public key configured")
	}

	server := server.NewHTTPServer(context.Background(), deps.ExecutorController, apiSignatureVerifier)

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	if err := server.Listen(cfg.HTTPAddress, fiber.ListenConfig{
		GracefulContext:       ctx,
		DisableStartupMessage: true,
	}); err != nil {
		log.Error().Err(err).Msg("HTTP server failed")
	}

	log.Info().Msg("Executor service stopped")
}

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flowbaker/flowbaker/internal/auth"
	"github.com/flowbaker/flowbaker/internal/initialization"
	"github.com/flowbaker/flowbaker/internal/server"
	"github.com/flowbaker/flowbaker/internal/version"
	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Define help flag
	var showHelp bool
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message")
	flag.Parse()

	if showHelp {
		printUsage()
		os.Exit(0)
	}

	args := flag.Args()

	// Default behavior: auto-start if configured, setup if not
	if len(args) == 0 {
		startExecutor()
		return
	}

	switch args[0] {
	case "start":
		startExecutor()
	case "reset":
		resetExecutor()
	case "status":
		showStatus()
	default:
		log.Error().Str("command", args[0]).Msg("Unknown command")
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`Usage: %s <command>

Commands:
  start                  Start the executor (auto-setup if needed)
  reset                  Reset configuration and start fresh
  status                 Show current executor status

Examples:
  %s start              # Start executor with auto-setup
  %s reset              # Start completely fresh

Environment Variables (optional):
  FLOWBAKER_API_URL              Override API URL (default: https://api.flowbaker.io)
  FLOWBAKER_DEBUG                Show detailed logs

`, os.Args[0], os.Args[0], os.Args[0])
}

func startExecutor() {
	if !initialization.IsSetupComplete() {
		// Start HTTP server with health check endpoint before registration
		// so the API can verify connectivity during the registration process
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			startHealthCheckServer(ctx)
		}()

		if _, err := initialization.RunFirstTimeSetup(); err != nil {
			log.Fatal().Err(err).Msg("Failed to complete setup")
		}

		cancel()

		// Give the server a moment to shutdown gracefully
		time.Sleep(100 * time.Millisecond)
	}

	runExecutor()
}

func resetExecutor() {
	if err := initialization.ResetConfig(); err != nil {
		log.Fatal().Err(err).Msg("Failed to reset configuration")
	}
	fmt.Println("✅ Configuration reset successfully")
	fmt.Printf("Run '%s start' to begin setup\n", os.Args[0])
}

func showStatus() {
	if initialization.IsSetupComplete() {
		config, err := initialization.LoadConfig()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to load configuration")
		}
		fmt.Println("✅ Executor is set up and ready")
		fmt.Printf("   Executor ID: %s\n", config.ExecutorID)
		fmt.Printf("   Workspace ID: %s\n", config.WorkspaceID)
		fmt.Printf("   API URL: %s\n", config.APIBaseURL)
		if !config.LastConnected.IsZero() {
			fmt.Printf("   Last connected: %s\n", config.LastConnected.Format("2006-01-02 15:04:05"))
		}
	} else {
		fmt.Println("❌ Executor is not set up")
		fmt.Printf("Run '%s start' to begin setup\n", os.Args[0])
	}
}

func runExecutor() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info().Msg("Starting executor service")

	config, err := initialization.LoadConfig()
	if err != nil || config == nil {
		log.Fatal().Msg("No configuration found. Run with 'start' to set up.")
	}

	log.Info().
		Str("executor_id", config.ExecutorID).
		Str("api_base_url", config.APIBaseURL).
		Msg("Executor configuration loaded")

	flowbakerClient := flowbaker.NewClient(
		flowbaker.WithBaseURL(config.APIBaseURL),
		flowbaker.WithExecutorID(config.ExecutorID),
		flowbaker.WithEd25519PrivateKey(config.Keys.Ed25519Private),
	)

	log.Info().Msg("Flowbaker client with signature-based auth ready")

	deps, err := BuildExecutorDependencies(context.Background(), ExecutorDependencyConfig{
		FlowbakerClient: flowbakerClient,
		ExecutorID:      config.ExecutorID,
		Config:          config,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to build executor dependencies")
	}

	// Initialize API signature verification with the public key from config
	var apiSignatureVerifier *auth.APISignatureVerifier
	if config.APIPublicKey != "" {
		var err error
		apiSignatureVerifier, err = auth.NewAPISignatureVerifier(config.APIPublicKey)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize API signature verifier")
		}
		log.Info().Msg("API signature verification enabled")
	} else {
		log.Warn().Msg("API signature verification disabled - no public key in config")
	}

	server := server.NewHTTPServer(context.Background(), deps.ExecutorController, apiSignatureVerifier)

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	if err := server.Listen(":8081", fiber.ListenConfig{
		GracefulContext:       ctx,
		DisableStartupMessage: true,
	}); err != nil {
		log.Error().Err(err).Msg("HTTP server failed")
	}

	log.Info().Msg("Executor service stopped")
}

// startHealthCheckServer starts a minimal HTTP server with only the health check endpoint
// This is used during the initial setup phase to allow API connectivity verification
func startHealthCheckServer(ctx context.Context) {
	app := fiber.New(fiber.Config{
		AppName: "flowbaker-executor-setup",
	})

	app.Use(cors.New())
	app.Use(logger.New())

	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":     "healthy",
			"service":    "flowbaker-executor",
			"version":    version.GetVersion(),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"setup_mode": true,
		})
	})

	if err := app.Listen(":8081", fiber.ListenConfig{
		GracefulContext:       ctx,
		DisableStartupMessage: true,
	}); err != nil {
		log.Error().Err(err).Msg("Health check server failed to start")
	}
}

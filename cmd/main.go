package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/flowbaker/flowbaker/internal/auth"
	"github.com/flowbaker/flowbaker/internal/initialization"
	"github.com/flowbaker/flowbaker/internal/managers"
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
	case "workspaces":
		handleWorkspaceCommands(args[1:])
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
  workspaces <subcommand> Manage workspace assignments

Workspace Commands:
  workspaces list        List all assigned workspaces
  workspaces add         Register executor with a new workspace (requires web verification)
  workspaces remove      Remove executor from a workspace

Examples:
  %s start              # Start executor with auto-setup
  %s reset              # Start completely fresh
  %s workspaces list    # List assigned workspaces
  %s workspaces add     # Add to new workspace

Environment Variables (optional):
  FLOWBAKER_API_URL              Override API URL (default: https://api.flowbaker.io)
  FLOWBAKER_DEBUG                Show detailed logs

`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
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
		fmt.Printf("   Workspaces (%d): %s\n", len(config.WorkspaceIDs), strings.Join(config.WorkspaceIDs, ", "))
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

// handleWorkspaceCommands handles all workspace-related subcommands
func handleWorkspaceCommands(args []string) {
	if len(args) == 0 {
		fmt.Println("Missing workspace subcommand. Available commands:")
		fmt.Println("  list    - List all assigned workspaces")
		fmt.Println("  add     - Add executor to a new workspace")
		fmt.Println("  remove  - Remove executor from a workspace")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		listWorkspaces()
	case "add":
		addWorkspace(args[1:])
	case "remove":
		removeWorkspace(args[1:])
	default:
		fmt.Printf("Unknown workspace command: %s\n", args[0])
		fmt.Println("Available commands: list, add, remove")
		os.Exit(1)
	}
}

// listWorkspaces shows all currently assigned workspaces
func listWorkspaces() {
	if !initialization.IsSetupComplete() {
		fmt.Println("❌ Executor is not set up. Run 'start' to begin setup.")
		os.Exit(1)
	}

	config, err := initialization.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	fmt.Println("📋 Assigned Workspaces:")
	if len(config.WorkspaceIDs) == 0 {
		fmt.Println("   No workspaces assigned")
		return
	}

	flowbakerClient := flowbaker.NewClient(
		flowbaker.WithBaseURL(config.APIBaseURL),
		flowbaker.WithExecutorID(config.ExecutorID),
		flowbaker.WithEd25519PrivateKey(config.Keys.Ed25519Private),
	)

	workspaceManager := managers.NewExecutorWorkspaceManager(managers.ExecutorWorkspaceManagerDependencies{
		FlowbakerClient: flowbakerClient,
	})

	ctx := context.Background()
	workspaces, err := workspaceManager.GetWorkspaces(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch workspace details")
	}

	for i, workspace := range workspaces {
		fmt.Printf("   %d. %s (%s)\n", i+1, workspace.Name, workspace.Slug)
	}
	fmt.Printf("\nTotal: %d workspace(s)\n", len(workspaces))
}

// addWorkspace adds the executor to a new workspace using the registration flow
func addWorkspace(_ []string) {
	if !initialization.IsSetupComplete() {
		fmt.Println("❌ Executor is not set up. Run 'start' to begin setup.")
		os.Exit(1)
	}

	_, err := initialization.AddWorkspace()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to add workspace")
	}
}

// removeWorkspace removes the executor from a workspace (placeholder for now)
func removeWorkspace(_ []string) {
	if !initialization.IsSetupComplete() {
		fmt.Println("❌ Executor is not set up. Run 'start' to begin setup.")
		os.Exit(1)
	}

	fmt.Println("🚧 Removing workspace assignments is not yet implemented.")
	fmt.Println("This feature will allow you to unregister this executor from workspaces.")
	fmt.Println("For now, use the web interface to manage workspace assignments.")
}

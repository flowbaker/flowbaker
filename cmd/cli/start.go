package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flowbaker/flowbaker/internal/controllers"
	"github.com/flowbaker/flowbaker/internal/initialization"
	"github.com/flowbaker/flowbaker/internal/middlewares"
	"github.com/flowbaker/flowbaker/internal/server"
	"github.com/flowbaker/flowbaker/internal/version"
	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func NewStartCommand(executorContainer *initialization.ExecutorContainer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the executor (auto-setup if needed)",
		Long:  `Start the executor service. If this is the first time running, it will automatically guide you through the setup process.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(executorContainer)
		},
	}

	return cmd
}

func runStart(executorContainer *initialization.ExecutorContainer) error {
	ctx := context.Background()
	configManager := executorContainer.GetConfigManager()
	workspaceRegistrationManager := executorContainer.GetWorkspaceRegistrationManager()

	if !configManager.IsSetupComplete(ctx) {
		// Start HTTP server with health check endpoint before registration
		// so the API can verify connectivity during the registration process
		ctx, cancel := context.WithCancel(ctx)

		go func() {
			startHealthCheckServer(ctx, executorContainer)
		}()

		if err := initialization.RunFirstTimeSetup(ctx, initialization.RunFirstTimeSetupParams{
			ConfigManager:       configManager,
			RegistrationManager: workspaceRegistrationManager,
		}); err != nil {
			log.Fatal().Err(err).Msg("Failed to complete setup")
		}

		cancel()

		// Give the server a moment to shutdown gracefully
		time.Sleep(100 * time.Millisecond)
	}

	return runExecutor(executorContainer)
}

func runExecutor(executorContainer *initialization.ExecutorContainer) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info().Msg("Starting executor service")

	configManager := executorContainer.GetConfigManager()
	config, err := configManager.GetConfig(ctx)
	if err != nil || config.ExecutorID == "" || !config.SetupComplete {
		log.Fatal().Msg("No configuration found. Run 'start' to set up.")
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

	deps, err := executorContainer.BuildExecutorDependencies(context.Background(), initialization.ExecutorDependencyConfig{
		FlowbakerClient: flowbakerClient,
		ExecutorID:      config.ExecutorID,
		Config:          config,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to build executor dependencies")
	}

	if len(config.Assignments) == 0 {
		log.Fatal().Msg("No workspace API keys found in config")
	}

	keyProvider := middlewares.NewConfigAPIKeyProvider(config)
	server := server.NewHTTPServer(context.Background(), deps.ExecutorController, keyProvider)

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	if err := server.Listen(":8081", fiber.ListenConfig{
		GracefulContext:       ctx,
		DisableStartupMessage: true,
	}); err != nil {
		log.Error().Err(err).Msg("HTTP server failed")
	}

	log.Info().Msg("Executor service stopped")
	return nil
}

// startHealthCheckServer starts a minimal HTTP server with only the health check endpoint
// This is used during the initial setup phase to allow API connectivity verification
func startHealthCheckServer(ctx context.Context, executorContainer *initialization.ExecutorContainer) {
	workspaceRegistrationManager := executorContainer.GetWorkspaceRegistrationManager()

	// Create a minimal executor controller for registration
	executorController := &controllers.ExecutorController{}
	executorController = controllers.NewExecutorController(controllers.ExecutorControllerDependencies{
		WorkspaceRegistrationManager: workspaceRegistrationManager,
	})

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

	workspaces := app.Group("/workspaces")
	workspaces.Post("/", executorController.RegisterWorkspace)

	if err := app.Listen(":8081", fiber.ListenConfig{
		GracefulContext:       ctx,
		DisableStartupMessage: true,
	}); err != nil {
		log.Error().Err(err).Msg("Health check server failed to start")
	}
}

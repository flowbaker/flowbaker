package server

import (
	"context"
	"os"
	"time"

	"github.com/flowbaker/flowbaker/internal/auth"
	"github.com/flowbaker/flowbaker/internal/controllers"
	"github.com/flowbaker/flowbaker/internal/middlewares"
	"github.com/flowbaker/flowbaker/internal/version"
	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/rs/zerolog/log"
)

type HTTPServerDependencies struct {
	Config             domain.ExecutorConfig
	ExecutorController *controllers.ExecutorController
	KeyProvider        middlewares.WorkspaceAPIKeyProvider
}

func NewHTTPServer(ctx context.Context, deps HTTPServerDependencies) *fiber.App {
	router := fiber.New(fiber.Config{
		AppName: "flowbaker-executor",
	})

	// Add basic middleware
	router.Use(cors.New())
	router.Use(logger.New())

	// Health check endpoint (no authentication required)
	router.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":    "healthy",
			"service":   "flowbaker-executor",
			"version":   version.GetVersion(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	if deps.Config.EnableWorkspaceRegistration {
		workspaces := router.Group("/workspaces")

		workspaces.Post("/", deps.ExecutorController.RegisterWorkspace)
	}

	specificWorkspace := router.Group("/workspaces/:workspaceID")

	staticAPIPublicKey := os.Getenv("STATIC_API_SIGNATURE_PUBLIC_KEY")

	if staticAPIPublicKey != "" {
		verifier, err := auth.NewAPISignatureVerifier(staticAPIPublicKey)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create API signature verifier")
		}

		specificWorkspace.Use(middlewares.APISignatureMiddleware(verifier))
	} else if deps.KeyProvider != nil {
		specificWorkspace.Use(middlewares.WorkspaceAwareAPISignatureMiddleware(deps.KeyProvider))
	} else {
		log.Fatal().Msg("No signature verification method available")
	}

	specificWorkspace.Post("/executions", deps.ExecutorController.StartExecution)
	specificWorkspace.Post("/executions/:executionID/nodes/:nodeID", deps.ExecutorController.RerunNode)
	specificWorkspace.Post("/polling-events", deps.ExecutorController.HandlePollingEvent)
	specificWorkspace.Post("/connection-test", deps.ExecutorController.TestConnection)
	specificWorkspace.Post("/peek-data", deps.ExecutorController.PeekData)
	specificWorkspace.Delete("/", deps.ExecutorController.UnregisterWorkspace)

	return router
}

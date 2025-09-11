package server

import (
	"context"
	"time"

	"github.com/flowbaker/flowbaker/internal/controllers"
	"github.com/flowbaker/flowbaker/internal/middlewares"
	"github.com/flowbaker/flowbaker/internal/version"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/rs/zerolog/log"
)

func NewHTTPServer(ctx context.Context, executorController *controllers.ExecutorController, keyProvider middlewares.WorkspaceAPIKeyProvider) *fiber.App {
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

	workspaces := router.Group("/workspaces")

	workspaces.Post("/", executorController.RegisterWorkspace)

	specificWorkspace := router.Group("/workspaces/:workspaceID")

	if keyProvider == nil {
		log.Fatal().Msg("Key provider is nil, please set up the executor with a key provider")
	}

	specificWorkspace.Use(middlewares.WorkspaceAwareAPISignatureMiddleware(keyProvider))

	specificWorkspace.Post("/executions", executorController.StartExecution)
	specificWorkspace.Post("/polling-events", executorController.HandlePollingEvent)
	specificWorkspace.Post("/connection-test", executorController.TestConnection)
	specificWorkspace.Post("/peek-data", executorController.PeekData)

	return router
}

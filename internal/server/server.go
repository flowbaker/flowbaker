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

	workspaces := router.Group("/workspaces/:workspaceID")

	if keyProvider == nil {
		log.Fatal().Msg("Key provider is nil, please set up the executor with a key provider")
	}

	workspaces.Use(middlewares.WorkspaceAwareAPISignatureMiddleware(keyProvider))

	workspaces.Post("/executions", executorController.StartExecution)
	workspaces.Post("/polling-events", executorController.HandlePollingEvent)
	workspaces.Post("/connection-test", executorController.TestConnection)
	workspaces.Post("/peek-data", executorController.PeekData)

	return router
}

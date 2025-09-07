package server

import (
	"context"
	"time"

	"github.com/flowbaker/flowbaker/internal/auth"
	"github.com/flowbaker/flowbaker/internal/controllers"
	"github.com/flowbaker/flowbaker/internal/middlewares"
	"github.com/flowbaker/flowbaker/internal/version"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/rs/zerolog/log"
)

func NewHTTPServer(ctx context.Context, executorController *controllers.ExecutorController, apiSignatureVerifier *auth.APISignatureVerifier) *fiber.App {
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

	// Add API signature verification middleware if verifier is available
	if apiSignatureVerifier != nil {
		router.Use(middlewares.APISignatureMiddleware(apiSignatureVerifier))
		log.Info().Msg("API signature verification middleware enabled")
	} else {
		log.Warn().Msg("API signature verification middleware disabled")
	}

	router.Post("/executions", executorController.StartExecution)
	router.Post("/polling-events", executorController.HandlePollingEvent)
	router.Post("/connection-test", executorController.TestConnection)
	router.Post("/peek-data", executorController.PeekData)

	return router
}

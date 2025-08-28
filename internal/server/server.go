package server

import (
	"context"
	"flowbaker/internal/auth"
	"flowbaker/internal/controllers"
	"flowbaker/internal/middlewares"

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

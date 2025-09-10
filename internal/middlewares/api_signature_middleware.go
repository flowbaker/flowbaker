package middlewares

import (
	"bytes"
	"io"

	"github.com/flowbaker/flowbaker/internal/auth"
	"github.com/flowbaker/flowbaker/internal/initialization"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

type WorkspaceAPIKeyProvider interface {
	GetWorkspaceAPIKey(workspaceID string) (string, error)
}

type ConfigAPIKeyProvider struct {
	config *initialization.ExecutorConfig
}

func NewConfigAPIKeyProvider(config *initialization.ExecutorConfig) *ConfigAPIKeyProvider {
	return &ConfigAPIKeyProvider{config: config}
}

func (p *ConfigAPIKeyProvider) GetWorkspaceAPIKey(workspaceID string) (string, error) {
	for _, wsKey := range p.config.WorkspaceAPIKeys {
		if wsKey.WorkspaceID == workspaceID {
			return wsKey.APIPublicKey, nil
		}
	}

	return "", fiber.NewError(fiber.StatusUnauthorized, "No API key found for workspace")
}

func WorkspaceAwareAPISignatureMiddleware(keyProvider WorkspaceAPIKeyProvider) fiber.Handler {
	return func(c fiber.Ctx) error {
		workspaceID := c.Params("workspaceID")
		if workspaceID == "" {
			log.Error().Str("path", c.Path()).Msg("No workspace ID found in path for signature verification")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Workspace ID required for signature verification",
			})
		}

		apiPublicKey, err := keyProvider.GetWorkspaceAPIKey(workspaceID)
		if err != nil {
			log.Error().
				Err(err).
				Str("workspace_id", workspaceID).
				Msg("Failed to get API public key for workspace")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid workspace or API key not found",
			})
		}

		verifier, err := auth.NewAPISignatureVerifier(apiPublicKey)
		if err != nil {
			log.Error().
				Err(err).
				Str("workspace_id", workspaceID).
				Msg("Failed to create signature verifier")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to initialize signature verification",
			})
		}

		signatureHeader := c.Get("X-API-Signature")
		timestampHeader := c.Get("X-API-Timestamp")

		bodyBytes, err := io.ReadAll(bytes.NewReader(c.Body()))
		if err != nil {
			log.Error().Err(err).Msg("Failed to read request body for signature verification")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Failed to read request body",
			})
		}

		err = verifier.VerifyRequest(
			c.Method(),
			c.Path(),
			signatureHeader,
			timestampHeader,
			bodyBytes,
		)

		if err != nil {
			log.Error().
				Err(err).
				Str("path", c.Path()).
				Str("method", c.Method()).
				Str("workspace_id", workspaceID).
				Str("signature", signatureHeader).
				Str("timestamp", timestampHeader).
				Msg("API signature verification failed")

			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API signature",
			})
		}

		log.Debug().
			Str("path", c.Path()).
			Str("method", c.Method()).
			Str("workspace_id", workspaceID).
			Msg("API signature verified successfully")

		return c.Next()
	}
}

func APISignatureMiddleware(verifier *auth.APISignatureVerifier) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get signature and timestamp headers
		signatureHeader := c.Get("X-API-Signature")
		timestampHeader := c.Get("X-API-Timestamp")

		// Read the request body
		bodyBytes, err := io.ReadAll(bytes.NewReader(c.Body()))
		if err != nil {
			log.Error().Err(err).Msg("Failed to read request body for signature verification")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Failed to read request body",
			})
		}

		// Verify the signature
		err = verifier.VerifyRequest(
			c.Method(),
			c.Path(),
			signatureHeader,
			timestampHeader,
			bodyBytes,
		)

		if err != nil {
			log.Error().
				Err(err).
				Str("path", c.Path()).
				Str("method", c.Method()).
				Str("signature", signatureHeader).
				Str("timestamp", timestampHeader).
				Msg("API signature verification failed")

			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API signature",
			})
		}

		log.Debug().
			Str("path", c.Path()).
			Str("method", c.Method()).
			Msg("API signature verified successfully")

		return c.Next()
	}
}

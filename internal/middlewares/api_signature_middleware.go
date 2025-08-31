package middlewares

import (
	"bytes"
	"io"

	"github.com/flowbaker/flowbaker/internal/auth"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

// APISignatureMiddleware creates middleware that verifies API request signatures
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

package stripe

import (
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/balance"
)

type StripeConnectionTester struct {
	credentialGetter domain.CredentialGetter[StripeCredential]
}

func NewStripeConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &StripeConnectionTester{
		credentialGetter: managers.NewExecutorCredentialGetter[StripeCredential](deps.ExecutorCredentialManager),
	}
}

func (c *StripeConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	// Validate credential type
	if params.Credential.Type != domain.CredentialTypeDefault {
		log.Error().Str("credential_type", string(params.Credential.Type)).Msg("Invalid credential type for Stripe - Basic credentials required")
		return false, fmt.Errorf("invalid credential type %s - Stripe requires Basic credentials", params.Credential.Type)
	}

	// Parse credential to extract secret key
	data, err := json.Marshal(params.Credential.DecryptedPayload)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal credential payload")
		return false, fmt.Errorf("failed to marshal credential payload: %w", err)
	}

	var stripeCredential StripeCredential
	if err := json.Unmarshal(data, &stripeCredential); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal Stripe credential")
		return false, fmt.Errorf("failed to unmarshal Stripe credential: %w", err)
	}

	// Validate that secret key is provided
	if stripeCredential.SecretKey == "" {
		log.Error().Msg("Stripe secret key is missing")
		return false, fmt.Errorf("Stripe secret key is required")
	}

	// Test the connection by making a simple API call to get balance
	if err := c.testStripeAPI(ctx, stripeCredential.SecretKey); err != nil {
		log.Error().Err(err).Msg("Failed to connect to Stripe API")
		return false, fmt.Errorf("failed to connect to Stripe API: %w", err)
	}

	return true, nil
}

func (c *StripeConnectionTester) testStripeAPI(ctx context.Context, secretKey string) error {
	// Set the API key for stripe-go
	stripe.Key = secretKey

	// Test the connection by getting balance using stripe-go
	_, err := balance.Get(nil)
	if err != nil {
		return fmt.Errorf("stripe API test failed: %w", err)
	}

	// Successfully connected
	return nil
}

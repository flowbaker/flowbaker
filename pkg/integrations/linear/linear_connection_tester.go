package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
)

type LinearConnectionTester struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewLinearConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &LinearConnectionTester{
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *LinearConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	log.Info().Msg("Testing connection to Linear API")

	// Linear uses OAuth credentials, so we need to get the OAuth account
	// using the credential ID (the service will handle the OAuth account lookup)
	if params.Credential.Type != domain.CredentialTypeOAuth && params.Credential.Type != domain.CredentialTypeOAuthWithParams {
		log.Error().Str("credential_type", string(params.Credential.Type)).Msg("Invalid credential type for Linear - OAuth required")
		return false, fmt.Errorf("invalid credential type %s - Linear requires OAuth authentication", params.Credential.Type)
	}

	// Get the OAuth account using the credential ID (not the OAuth account ID)
	oauthAccount, err := c.credentialGetter.GetDecryptedCredential(ctx, params.Credential.ID)
	if err != nil {
		log.Error().Err(err).Str("credential_id", params.Credential.ID).Msg("Failed to get decrypted Linear OAuth credential")
		return false, fmt.Errorf("failed to get decrypted Linear OAuth credential: %w", err)
	}

	// Test the connection by getting the authenticated user
	err = c.testLinearAPI(ctx, oauthAccount.AccessToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to Linear API")
		return false, fmt.Errorf("failed to connect to Linear API: %w", err)
	}

	log.Info().Msg("Successfully connected to Linear API")
	return true, nil
}

func (c *LinearConnectionTester) testLinearAPI(ctx context.Context, accessToken string) error {
	// Linear GraphQL query to get current user info (same as in OAuth service)
	query := `
	query Viewer {
		viewer {
			id
			name
			email
		}
	}`

	// Prepare the GraphQL request
	requestBody := map[string]any{
		"query": query,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.linear.app/graphql", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request to Linear API: %w", err)
	}
	defer resp.Body.Close()

	// Check if response status is OK
	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status_code", resp.StatusCode).Msg("Linear API returned non-OK status")
		return fmt.Errorf("Linear API returned status %d", resp.StatusCode)
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the response to check for GraphQL errors
	var response struct {
		Data struct {
			Viewer struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors,omitempty"`
	}

	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return fmt.Errorf("failed to decode Linear API response: %w", err)
	}

	// Check for GraphQL errors
	if len(response.Errors) > 0 {
		return fmt.Errorf("Linear API returned errors: %v", response.Errors)
	}

	// Verify we got user data
	if response.Data.Viewer.ID == "" {
		return fmt.Errorf("Linear API did not return user information")
	}

	// Log successful connection with user info
	if response.Data.Viewer.Name != "" {
		log.Info().Str("user", response.Data.Viewer.Name).Msg("Successfully connected to Linear")
	} else {
		log.Info().Str("user_id", response.Data.Viewer.ID).Msg("Successfully connected to Linear")
	}

	return nil
}

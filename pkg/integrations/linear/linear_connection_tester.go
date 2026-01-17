package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
)

type Viewer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type GraphQLError struct {
	Message string `json:"message"`
}

type ViewerResponse struct {
	Data struct {
		Viewer Viewer `json:"viewer"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

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

	if params.Credential.Type != domain.CredentialTypeOAuth && params.Credential.Type != domain.CredentialTypeOAuthWithParams {
		log.Error().Str("credential_type", string(params.Credential.Type)).Msg("Invalid credential type for Linear - OAuth required")
		return false, fmt.Errorf("invalid credential type %s - Linear requires OAuth authentication", params.Credential.Type)
	}

	oauthAccount, err := c.credentialGetter.GetDecryptedCredential(ctx, params.Credential.ID)
	if err != nil {
		log.Error().Err(err).Str("credential_id", params.Credential.ID).Msg("Failed to get decrypted Linear OAuth credential")
		return false, fmt.Errorf("failed to get decrypted Linear OAuth credential: %w", err)
	}

	err = c.testLinearAPI(ctx, oauthAccount.AccessToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to Linear API")
		return false, fmt.Errorf("failed to connect to Linear API: %w", err)
	}

	log.Info().Msg("Successfully connected to Linear API")
	return true, nil
}

func (c *LinearConnectionTester) testLinearAPI(ctx context.Context, accessToken string) error {
	query := `
	query Viewer {
		viewer {
			id
			name
			email
		}
	}`

	requestBody := map[string]any{
		"query": query,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.linear.app/graphql", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request to Linear API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status_code", resp.StatusCode).Msg("Linear API returned non-OK status")
		return fmt.Errorf("Linear API returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var response ViewerResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return fmt.Errorf("failed to decode Linear API response: %w", err)
	}

	if len(response.Errors) > 0 {
		return fmt.Errorf("Linear API returned errors: %v", response.Errors)
	}

	if response.Data.Viewer.ID == "" {
		return fmt.Errorf("Linear API did not return user information")
	}

	if response.Data.Viewer.Name != "" {
		log.Info().Str("user", response.Data.Viewer.Name).Msg("Successfully connected to Linear")
	} else {
		log.Info().Str("user_id", response.Data.Viewer.ID).Msg("Successfully connected to Linear")
	}

	return nil
}

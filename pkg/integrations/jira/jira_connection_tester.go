package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
)

type JiraConnectionTester struct {
	executorCredentialManager domain.ExecutorCredentialManager
	httpClient                *http.Client
}

func NewJiraConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &JiraConnectionTester{
		executorCredentialManager: deps.ExecutorCredentialManager,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *JiraConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	log.Info().Msg("Testing connection to Jira API")

	// Jira uses OAuth credentials, so we need to check for OAuth or OAuthWithParams types
	if params.Credential.Type != domain.CredentialTypeOAuth && params.Credential.Type != domain.CredentialTypeOAuthWithParams {
		log.Error().Str("credential_type", string(params.Credential.Type)).Msg("Invalid credential type for Jira - OAuth required")
		return false, fmt.Errorf("invalid credential type %s - Jira requires OAuth authentication", params.Credential.Type)
	}

	// Get the OAuth account using the credential ID
	credential, err := c.executorCredentialManager.GetFullCredential(ctx, params.Credential.ID)
	if err != nil {
		log.Error().Err(err).Str("credential_id", params.Credential.ID).Msg("Failed to get decrypted Jira OAuth credential")
		return false, fmt.Errorf("failed to get decrypted Jira OAuth credential: %w", err)
	}

	oauthAccount, err := c.executorCredentialManager.GetOAuthAccount(ctx, credential.OAuthAccountID)
	if err != nil {
		log.Error().Err(err).Str("credential_id", params.Credential.ID).Msg("Failed to get OAuth account")
		return false, fmt.Errorf("failed to get OAuth account: %w", err)
	}

	sensitiveData := domain.OAuthAccountSensitiveData{}

	sensitiveDataBytes, err := json.Marshal(credential.DecryptedPayload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal credential: %w", err)
	}

	err = json.Unmarshal(sensitiveDataBytes, &sensitiveData)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal credential: %w", err)
	}

	// Parse the structured metadata
	metadataBytes, err := json.Marshal(oauthAccount.Metadata)
	if err != nil {
		return false, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var jiraMetadata JiraOAuthMetadata
	if err := json.Unmarshal(metadataBytes, &jiraMetadata); err != nil {
		return false, fmt.Errorf("failed to unmarshal Jira metadata: %w", err)
	}

	// Try selected cloud ID first if available
	if jiraMetadata.SelectedCloudID != "" {
		log.Info().Str("selected_cloud_id", jiraMetadata.SelectedCloudID).Msg("Testing connection with selected cloud ID")

		// Find the selected resource
		for _, resource := range jiraMetadata.AccessibleResources {
			if resource.ID == jiraMetadata.SelectedCloudID {
				success, err := c.testJiraConnection(ctx, sensitiveData.AccessToken, resource.ID, resource.URL)
				if err == nil {
					log.Info().Str("cloud_id", resource.ID).Str("base_url", resource.URL).Msg("Successfully connected to Jira with selected cloud ID")
					return success, nil
				}
				log.Warn().Err(err).Str("cloud_id", resource.ID).Msg("Failed to connect with selected cloud ID, trying other resources")
				break
			}
		}
	}

	// Try all accessible resources if selected one failed or wasn't found
	if len(jiraMetadata.AccessibleResources) > 0 {
		log.Info().Int("resource_count", len(jiraMetadata.AccessibleResources)).Msg("Trying all accessible resources for connection test")

		var lastError error
		for _, resource := range jiraMetadata.AccessibleResources {
			log.Info().Str("cloud_id", resource.ID).Str("name", resource.Name).Msg("Testing connection with cloud ID")

			success, err := c.testJiraConnection(ctx, sensitiveData.AccessToken, resource.ID, resource.URL)
			if err == nil {
				log.Info().Str("cloud_id", resource.ID).Str("base_url", resource.URL).Msg("Successfully connected to Jira")
				return success, nil
			}

			log.Warn().Err(err).Str("cloud_id", resource.ID).Msg("Failed to connect, trying next resource")
			lastError = err
		}

		if lastError != nil {
			return false, fmt.Errorf("failed to connect to any accessible Jira resource: %w", lastError)
		}
	}

	// Last resort: try to retrieve accessible resources using the access token
	log.Info().Msg("No accessible resources in metadata, trying to retrieve them")
	retrievedResources, err := c.getAccessibleResourcesWithToken(ctx, sensitiveData.AccessToken)
	if err != nil {
		return false, fmt.Errorf("no accessible Jira resources found and failed to retrieve them: %w", err)
	}

	if len(retrievedResources) == 0 {
		return false, fmt.Errorf("no accessible Jira resources available for this OAuth account")
	}

	// Try the retrieved resources
	var lastError error
	for _, resource := range retrievedResources {
		log.Info().Str("cloud_id", resource.ID).Str("name", resource.Name).Msg("Testing connection with retrieved cloud ID")

		success, err := c.testJiraConnection(ctx, sensitiveData.AccessToken, resource.ID, resource.URL)
		if err == nil {
			log.Info().Str("cloud_id", resource.ID).Str("base_url", resource.URL).Msg("Successfully connected to Jira with retrieved resource")
			return success, nil
		}

		log.Warn().Err(err).Str("cloud_id", resource.ID).Msg("Failed to connect with retrieved resource")
		lastError = err
	}

	if lastError != nil {
		return false, fmt.Errorf("failed to connect to any retrieved Jira resource: %w", lastError)
	}

	return false, fmt.Errorf("no accessible Jira resources found")
}

// getAccessibleResourcesWithToken retrieves accessible resources using an access token
func (c *JiraConnectionTester) getAccessibleResourcesWithToken(ctx context.Context, accessToken string) ([]JiraAccessibleResource, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.atlassian.com/oauth/token/accessible-resources", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get accessible resources: status %d", resp.StatusCode)
	}

	var resources []JiraAccessibleResource
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return nil, err
	}

	return resources, nil
}

// testJiraConnection tests the Jira connection by getting current user information
func (c *JiraConnectionTester) testJiraConnection(ctx context.Context, accessToken, cloudID, baseURL string) (bool, error) {
	// Use Atlassian API gateway to get current user profile
	apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/myself", cloudID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Check if response status is OK
	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status_code", resp.StatusCode).Msg("Jira API returned non-OK status")
		return false, fmt.Errorf("Jira API returned status %d", resp.StatusCode)
	}

	// Parse response to verify we got valid user data
	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return false, fmt.Errorf("failed to parse user response: %w", err)
	}

	// Check if we received valid user information
	if accountID, exists := userInfo["accountId"]; exists {
		log.Info().Interface("account_id", accountID).Msg("Successfully authenticated with Jira")
		return true, nil
	}

	return false, fmt.Errorf("received invalid user information from Jira API")
}

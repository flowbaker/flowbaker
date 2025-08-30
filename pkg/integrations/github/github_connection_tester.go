package githubintegration

import (
	"context"
	"fmt"
	"net/http"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/internal/domain"

	"github.com/google/go-github/v57/github"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

type GitHubConnectionTester struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewGitHubConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &GitHubConnectionTester{
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *GitHubConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	// GitHub uses OAuth credentials, so we need to get the OAuth account
	// using the credential ID (the service will handle the OAuth account lookup)
	if params.Credential.Type != domain.CredentialTypeOAuth && params.Credential.Type != domain.CredentialTypeOAuthWithParams {
		log.Error().Str("credential_type", string(params.Credential.Type)).Msg("Invalid credential type for GitHub - OAuth required")
		return false, fmt.Errorf("invalid credential type %s - GitHub requires OAuth authentication", params.Credential.Type)
	}

	// Get the OAuth account using the credential ID (not the OAuth account ID)
	oauthAccount, err := c.credentialGetter.GetDecryptedCredential(ctx, params.Credential.ID)
	if err != nil {
		log.Error().Err(err).Str("credential_id", params.Credential.ID).Msg("Failed to get decrypted GitHub OAuth credential")
		return false, fmt.Errorf("failed to get decrypted GitHub OAuth credential: %w", err)
	}

	// Create OAuth2 token source using the OAuth account structure
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: oauthAccount.AccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	// Create GitHub client
	client := github.NewClient(tc)

	// Test the connection by getting the authenticated user
	user, response, err := client.Users.Get(ctx, "")
	if err != nil {
		log.Error().Err(err).Msg("Failed to get authenticated user from GitHub")
		return false, fmt.Errorf("failed to authenticate with GitHub: %w", err)
	}

	// Check if response status is OK
	if response.StatusCode != http.StatusOK {
		log.Error().Int("status_code", response.StatusCode).Msg("GitHub API returned non-OK status")
		return false, fmt.Errorf("GitHub API returned status %d", response.StatusCode)
	}

	// Log successful connection
	if user.Login != nil {
		log.Info().Str("user", *user.Login).Msg("Successfully connected to GitHub")
	} else {
		log.Info().Msg("Successfully connected to GitHub")
	}

	return true, nil
}

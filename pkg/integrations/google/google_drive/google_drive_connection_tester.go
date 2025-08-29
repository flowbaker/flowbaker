package googledrive

import (
	"context"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type GoogleDriveConnectionTester struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewGoogleDriveConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &GoogleDriveConnectionTester{
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *GoogleDriveConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	log.Info().Msg("Testing connection to Google Drive API")

	// Google Drive uses OAuth credentials
	if params.Credential.Type != domain.CredentialTypeOAuth && params.Credential.Type != domain.CredentialTypeOAuthWithParams {
		log.Error().Str("credential_type", string(params.Credential.Type)).Msg("Invalid credential type for Google Drive - OAuth required")
		return false, fmt.Errorf("invalid credential type %s - Google Drive requires OAuth authentication", params.Credential.Type)
	}

	// Get the OAuth account using the credential ID
	oauthAccount, err := c.credentialGetter.GetDecryptedCredential(ctx, params.Credential.ID)
	if err != nil {
		log.Error().Err(err).Str("credential_id", params.Credential.ID).Msg("Failed to get decrypted Google Drive OAuth credential")
		return false, fmt.Errorf("failed to get decrypted Google Drive OAuth credential: %w", err)
	}

	// Create OAuth2 token source using the OAuth account structure
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: oauthAccount.AccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	// Create Google Drive service
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(tc))
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Google Drive service")
		return false, fmt.Errorf("failed to create Google Drive service: %w", err)
	}

	// Test the connection by getting user information about the Drive
	about, err := driveService.About.Get().Fields("user(displayName,emailAddress),storageQuota").Do()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user info from Google Drive")
		return false, fmt.Errorf("failed to authenticate with Google Drive: %w", err)
	}

	// Verify we got valid user data
	if about.User == nil {
		log.Error().Msg("Google Drive API returned no user information")
		return false, fmt.Errorf("Google Drive API returned no user information")
	}

	// Log successful connection with user details
	userEmail := "unknown"
	userName := "unknown"
	if about.User.EmailAddress != "" {
		userEmail = about.User.EmailAddress
	}
	if about.User.DisplayName != "" {
		userName = about.User.DisplayName
	}

	log.Info().
		Str("user_name", userName).
		Str("user_email", userEmail).
		Msg("Successfully connected to Google Drive")

	// Additional verification: try to list files to ensure we have proper access
	// This is a lightweight check that verifies both authentication and permissions
	filesList := driveService.Files.List().PageSize(1).Fields("files(id,name)")
	_, err = filesList.Do()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list files from Google Drive - insufficient permissions")
		return false, fmt.Errorf("connection successful but insufficient permissions to access Google Drive files: %w", err)
	}

	log.Info().Msg("Google Drive connection test completed successfully - authentication and permissions verified")
	return true, nil
}

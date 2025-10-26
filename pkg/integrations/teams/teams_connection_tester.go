package teams

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
	auth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/rs/zerolog/log"
)

type TeamsConnectionTester struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewTeamsConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &TeamsConnectionTester{
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *TeamsConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	if params.Credential.Type != domain.CredentialTypeOAuth && params.Credential.Type != domain.CredentialTypeOAuthWithParams {
		log.Error().Str("credential_type", string(params.Credential.Type)).Msg("Invalid credential type for Teams - OAuth required")
		return false, fmt.Errorf("invalid credential type %s - Teams requires OAuth authentication", params.Credential.Type)
	}

	oauthAccount, err := c.credentialGetter.GetDecryptedCredential(ctx, params.Credential.ID)
	if err != nil {
		log.Error().Err(err).Str("credential_id", params.Credential.ID).Msg("Failed to get decrypted Teams OAuth credential")
		return false, fmt.Errorf("failed to get decrypted Teams OAuth credential: %w", err)
	}

	credential := &TeamsTokenCredential{accessToken: oauthAccount.AccessToken}
	authProvider, err := auth.NewAzureIdentityAuthenticationProviderWithScopes(credential, []string{"https://graph.microsoft.com/.default"})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create auth provider")
		return false, fmt.Errorf("failed to create auth provider: %w", err)
	}

	adapter, err := msgraphsdk.NewGraphRequestAdapter(authProvider)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Graph request adapter")
		return false, fmt.Errorf("failed to create Graph request adapter: %w", err)
	}

	graphClient := msgraphsdk.NewGraphServiceClient(adapter)

	user, err := graphClient.Me().Get(ctx, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user profile from Microsoft Graph")
		return false, fmt.Errorf("failed to authenticate with Microsoft Teams: %w", err)
	}

	if user.GetDisplayName() != nil {
		log.Info().Str("user", *user.GetDisplayName()).Msg("Successfully connected to Microsoft Teams")
	} else {
		log.Info().Msg("Successfully connected to Microsoft Teams")
	}

	return true, nil
}

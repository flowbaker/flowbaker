package gitlab

import (
	"context"
	"fmt"
	"net/http"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/xanzy/go-gitlab"
)

type GitLabConnectionTester struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewGitLabConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &GitLabConnectionTester{
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *GitLabConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	if params.Credential.Type != domain.CredentialTypeOAuth && params.Credential.Type != domain.CredentialTypeOAuthWithParams {
		return false, fmt.Errorf("invalid credential type %s - GitLab requires OAuth authentication", params.Credential.Type)
	}

	oauthAccount, err := c.credentialGetter.GetDecryptedCredential(ctx, params.Credential.ID)
	if err != nil {
		return false, fmt.Errorf("failed to get decrypted GitLab OAuth credential: %w", err)
	}

	client, err := gitlab.NewOAuthClient(oauthAccount.AccessToken)
	if err != nil {
		return false, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	_, response, err := client.Users.CurrentUser()
	if err != nil {
		return false, fmt.Errorf("failed to authenticate with GitLab: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("GitLab API returned status %d", response.StatusCode)
	}

	return true, nil
}

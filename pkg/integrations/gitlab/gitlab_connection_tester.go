package gitlab

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitLabConnectionTester struct {
	credentialGetter domain.CredentialGetter[GitLabCredential]
}

func NewGitLabConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &GitLabConnectionTester{
		credentialGetter: managers.NewExecutorCredentialGetter[GitLabCredential](deps.ExecutorCredentialManager),
	}
}

func (t *GitLabConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	credential, err := t.credentialGetter.GetDecryptedCredential(ctx, params.Credential.ID)
	if err != nil {
		return false, fmt.Errorf("failed to get decrypted GitLab credential: %w", err)
	}

	if credential.BaseURL == "" {
		credential.BaseURL = "https://gitlab.com"
	}

	var client *gitlab.Client

	if credential.BaseURL == "https://gitlab.com" {
		client, err = gitlab.NewClient(credential.APIToken)
	} else {
		client, err = gitlab.NewClient(credential.APIToken, gitlab.WithBaseURL(credential.BaseURL))
	}
	if err != nil {
		return false, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	_, _, err = client.Users.CurrentUser(gitlab.WithContext(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to authenticate with GitLab: %w", err)
	}

	return true, nil
}

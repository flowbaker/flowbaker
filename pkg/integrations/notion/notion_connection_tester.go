package notionintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

type NotionConnectionTester struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewNotionConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &NotionConnectionTester{
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *NotionConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	if params.Credential.Type != domain.CredentialTypeOAuth && params.Credential.Type != domain.CredentialTypeOAuthWithParams {
		return false, fmt.Errorf("invalid credential type %s - Notion requires OAuth authentication", params.Credential.Type)
	}

	oauthAccount, err := c.credentialGetter.GetDecryptedCredential(ctx, params.Credential.ID)
	if err != nil {
		return false, fmt.Errorf("failed to get decrypted Notion OAuth credential: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.notion.com/v1/users/me", nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+oauthAccount.AccessToken)
	req.Header.Set("Notion-Version", "2022-06-28")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to authenticate with Notion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("Notion API returned status %d: %s", resp.StatusCode, string(body))
	}

	var user struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return false, fmt.Errorf("failed to parse user response: %w", err)
	}

	if user.ID == "" {
		return false, fmt.Errorf("notion API returned invalid user data")
	}

	return true, nil
}

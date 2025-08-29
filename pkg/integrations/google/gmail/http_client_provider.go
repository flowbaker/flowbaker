package gmail

import (
	"context"
	"flowbaker/internal/domain"
	"net/http"

	"golang.org/x/oauth2"
)

type GmailHTTPClientProvider struct {
}

func NewGmailHTTPClientProvider(deps domain.IntegrationDeps) domain.HTTPOauthClientProvider {
	return &GmailHTTPClientProvider{}
}

func NewGmailClient(credential *domain.OAuthAccountWithSensitiveData) (*http.Client, error) {
	config := oauth2.Config{}
	token := &oauth2.Token{
		AccessToken: credential.SensitiveData.AccessToken,
		TokenType:   "Bearer",
	}

	httpClient := config.Client(context.TODO(), token)

	return httpClient, nil
}

func (p *GmailHTTPClientProvider) GetHTTPOAuthClient(credential *domain.OAuthAccountWithSensitiveData) (*http.Client, error) {
	gmailClient, err := NewGmailClient(credential)
	if err != nil {
		return nil, err
	}

	return gmailClient, nil
}

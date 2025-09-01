package dropbox

import (
	"context"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

type DropboxHTTPClientProvider struct {
}

func NewDropboxHTTPClientProvider(deps domain.IntegrationDeps) domain.HTTPOauthClientProvider {
	return &DropboxHTTPClientProvider{}
}

func NewDropboxClient(credential *domain.OAuthAccountWithSensitiveData) (*http.Client, error) {
	config := oauth2.Config{}
	token := &oauth2.Token{
		AccessToken: credential.SensitiveData.AccessToken,
		TokenType:   "Bearer",
	}

	log.Info().Msgf("token: %+v", token)

	httpClient := config.Client(context.TODO(), token)

	return httpClient, nil
}

func (p *DropboxHTTPClientProvider) GetHTTPOAuthClient(credential *domain.OAuthAccountWithSensitiveData) (*http.Client, error) {
	dropboxClient, err := NewDropboxClient(credential)
	if err != nil {
		return nil, err
	}

	return dropboxClient, nil
}

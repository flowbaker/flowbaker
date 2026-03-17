package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type HTTPClientManager interface {
	GetHTTPClient(ctx context.Context, p GetHTTPCredentialClientParams) (*http.Client, error)
}

type HTTPClientManagerDependencies struct {
	HTTPCredentialGetter      domain.CredentialGetter[HTTPDecryptionResult]
	IntegrationSelector       domain.IntegrationSelector
	ExecutorCredentialManager domain.ExecutorCredentialManager
	CredentialID              string
}

type httpClientManager struct {
	httpCredentialGetter      domain.CredentialGetter[HTTPDecryptionResult]
	integrationSelector       domain.IntegrationSelector
	executorCredentialManager domain.ExecutorCredentialManager
	credentialID              string
}

func NewHTTPClientManager(deps HTTPClientManagerDependencies) HTTPClientManager {
	return &httpClientManager{
		httpCredentialGetter:      deps.HTTPCredentialGetter,
		integrationSelector:       deps.IntegrationSelector,
		executorCredentialManager: deps.ExecutorCredentialManager,
		credentialID:              deps.CredentialID,
	}
}

func (m *httpClientManager) GetHTTPClient(ctx context.Context, p GetHTTPCredentialClientParams) (*http.Client, error) {
	switch p.AuthType {
	case HTTPAuthType_Generic:
		return m.getGenericClient(ctx)

	case HTTPAuthType_PreDefined:
		credential, err := m.executorCredentialManager.GetFullCredential(ctx, m.credentialID)
		if err != nil {
			return nil, err
		}

		return m.getPredefinedClient(ctx, credential)

	default:
		return m.getNoCredentialClient()
	}
}

func (m *httpClientManager) getNoCredentialClient() (*http.Client, error) {
	return &http.Client{}, nil
}

func (m *httpClientManager) getGenericClient(ctx context.Context) (*http.Client, error) {
	decryptionResult, err := m.httpCredentialGetter.GetDecryptedCredential(ctx, m.credentialID)
	if err != nil {
		return nil, err
	}

	genericHTTPClientProvider := NewHTTPClientProviderGeneric()

	client, err := genericHTTPClientProvider.GetHTTPDefaultClientGeneric(ctx, decryptionResult)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (m *httpClientManager) getPredefinedClient(ctx context.Context, credentialRaw domain.Credential) (*http.Client, error) {
	switch credentialRaw.Type {
	case domain.CredentialTypeOAuth, domain.CredentialTypeOAuthWithParams:
		return m.getPreDefinedOAuthClient(ctx, credentialRaw.IntegrationType, credentialRaw)

	case domain.CredentialTypeDefault:
		return m.getPreDefinedDefaultClient(ctx, credentialRaw.IntegrationType, credentialRaw)
	}

	return nil, fmt.Errorf("unsupported credential type for http client provider: %s", credentialRaw.Type)
}

func (m *httpClientManager) getPreDefinedOAuthClient(ctx context.Context, integrationTypeCredential domain.IntegrationType, credential domain.Credential) (*http.Client, error) {
	oauthAccount, err := m.executorCredentialManager.GetOAuthAccount(ctx, credential.OAuthAccountID)
	if err != nil {
		return nil, err
	}

	httpClient, err := m.integrationSelector.SelectHTTPOAuthClientProvider(ctx, domain.SelectIntegrationParams{
		IntegrationType: integrationTypeCredential,
	})
	if err != nil {
		return nil, err
	}

	sensitiveData := domain.OAuthAccountSensitiveData{}

	sensitiveDataBytes, err := json.Marshal(credential.DecryptedPayload)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(sensitiveDataBytes, &sensitiveData)
	if err != nil {
		return nil, err
	}

	client, err := httpClient.GetHTTPOAuthClient(&domain.OAuthAccountWithSensitiveData{
		OAuthAccount:  oauthAccount,
		SensitiveData: sensitiveData,
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (m *httpClientManager) getPreDefinedDefaultClient(ctx context.Context, integrationTypeCredential domain.IntegrationType, credentialRaw domain.Credential) (*http.Client, error) {
	httpClient, err := m.integrationSelector.SelectHTTPDefaultClientProvider(ctx, domain.SelectIntegrationParams{
		IntegrationType: integrationTypeCredential,
	})
	if err != nil {
		return nil, err
	}

	var httpDecryptionResult HTTPDecryptionResult

	dataBytes, err := json.Marshal(credentialRaw.DecryptedPayload)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(dataBytes, &httpDecryptionResult)
	if err != nil {
		return nil, err
	}

	client, err := httpClient.GetHTTPDefaultClient(&credentialRaw)
	if err != nil {
		return nil, err
	}

	return client, nil
}

package http

import (
	"context"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type PreDefinedCredentialManager interface {
	Authenticate(ctx context.Context, req *http.Request, rawCredential domain.Credential) (*http.Request, error)
}

type preDefinedCredentialManager struct {
	executorCredentialManager domain.ExecutorCredentialManager
	integrationSelector       domain.IntegrationSelector
}

type PreDefinedCredentialManagerDependencies struct {
	ExecutorCredentialManager domain.ExecutorCredentialManager
	IntegrationSelector       domain.IntegrationSelector
}

func NewPreDefinedCredentialManager(deps PreDefinedCredentialManagerDependencies) PreDefinedCredentialManager {
	return &preDefinedCredentialManager{
		executorCredentialManager: deps.ExecutorCredentialManager,
		integrationSelector:       deps.IntegrationSelector,
	}
}

func (m *preDefinedCredentialManager) Authenticate(ctx context.Context, req *http.Request, rawCredential domain.Credential) (*http.Request, error) {
	provider, err := m.integrationSelector.SelectHTTPRequestProvider(ctx, domain.SelectIntegrationParams{
		IntegrationType: domain.IntegrationType(rawCredential.IntegrationType),
	})
	if err != nil {
		return nil, err
	}

	return provider.Attach(req, &rawCredential)
}

package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type ApplyCredentialParams struct {
	AuthType        HTTPAuthType
	GenericAuthType HTTPGenericAuthType
	Request         *http.Request     `json:"-"`
	RequestParams   HTTPRequestParams `json:"-"`
	Credential      domain.Credential `json:"-"`
}

type HTTPPayload struct {
	AuthType        HTTPAuthType        `json:"auth_type"`
	GenericAuthType HTTPGenericAuthType `json:"generic_auth_type"` // nil if auth type is not generic
	Credential      domain.Credential   `json:"-"`
}

type CredentialManager interface {
	Authenticate(ctx context.Context, params ApplyCredentialParams) (*http.Request, error)
	GetPayload(ctx context.Context) (HTTPPayload, error)
}

type CredentialManagerDependencies struct {
	ExecutorCredentialManager domain.ExecutorCredentialManager
	IntegrationSelector       domain.IntegrationSelector
	CredentialID              string
}

type credentialManager struct {
	executorCredentialManager   domain.ExecutorCredentialManager
	credentialID                string
	genericCredentialManager    GenericCredentialManager
	preDefinedCredentialManager PreDefinedCredentialManager
}

func NewCredentialManager(deps CredentialManagerDependencies) CredentialManager {
	genericCredentialManager := NewGenericCredentialManager(GenericCredentialManagerDependencies{
		ExecutorCredentialManager: deps.ExecutorCredentialManager,
	})
	preDefinedCredentialManager := NewPreDefinedCredentialManager(PreDefinedCredentialManagerDependencies{
		ExecutorCredentialManager: deps.ExecutorCredentialManager,
		IntegrationSelector:       deps.IntegrationSelector,
	})

	return &credentialManager{
		executorCredentialManager:   deps.ExecutorCredentialManager,
		credentialID:                deps.CredentialID,
		genericCredentialManager:    genericCredentialManager,
		preDefinedCredentialManager: preDefinedCredentialManager,
	}
}

type HTTPAuthType string

const (
	HTTPAuthType_NoCredential HTTPAuthType = "no_credential"
	HTTPAuthType_Generic      HTTPAuthType = "generic"
	HTTPAuthType_PreDefined   HTTPAuthType = "pre_defined"
)

func (m *credentialManager) GetPayload(ctx context.Context) (HTTPPayload, error) {
	rawCredential, err := m.executorCredentialManager.GetFullCredential(ctx, m.credentialID)
	if err != nil {
		return HTTPPayload{}, err
	}

	jsonPayload, err := json.Marshal(rawCredential.DecryptedPayload)
	if err != nil {
		return HTTPPayload{}, err
	}

	var payload HTTPPayload
	err = json.Unmarshal(jsonPayload, &payload)
	if err != nil {
		return HTTPPayload{}, err
	}

	payload.Credential = rawCredential
	return payload, nil
}

func (m *credentialManager) Authenticate(ctx context.Context, params ApplyCredentialParams) (*http.Request, error) {
	log.Debug().Msgf("Authenticating with credential: %+v", params.Credential)
	switch params.AuthType {
	case HTTPAuthType_NoCredential:
		return params.Request, nil

	case HTTPAuthType_Generic:
		return m.genericCredentialManager.Authenticate(ctx, params.Request, params.GenericAuthType, params.Credential)

	// credential itself doesn't know is it for http or not, so we set it to default case here
	// maybe we should fine better approach for this
	default:
		return m.preDefinedCredentialManager.Authenticate(ctx, params.Request, params.Credential)
	}
}

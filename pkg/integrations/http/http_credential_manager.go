package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type ApplyCredentialParams struct {
	AuthType           HTTPAuthType
	GenericAuthType    HTTPGenericAuthType
	PreDefinedAuthType any
	Request            *http.Request
	RequestParams      HTTPRequestParams
}

type HTTPPayload struct {
	AuthType           HTTPAuthType        `json:"auth_type"`
	GenericAuthType    HTTPGenericAuthType `json:"generic_auth_type"`
	PreDefinedAuthType any                 `json:"pre_defined_auth_type"`
	Credential         domain.Credential   `json:"-"`
}

type CredentialManager interface {
	Authenticate(ctx context.Context, params ApplyCredentialParams) (*http.Request, error)
	GetPayload(ctx context.Context) (HTTPPayload, error)
}

type CredentialManagerDependencies struct {
	ExecutorCredentialManager domain.ExecutorCredentialManager
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
		CredentialID:              deps.CredentialID,
	})
	preDefinedCredentialManager := NewPreDefinedCredentialManager(PreDefinedCredentialManagerDependencies{
		ExecutorCredentialManager: deps.ExecutorCredentialManager,
		CredentialID:              deps.CredentialID,
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

func (m *credentialManager) Authenticate(ctx context.Context, params ApplyCredentialParams) (*http.Request, error) {
	switch params.AuthType {
	case HTTPAuthType_NoCredential:
		return params.Request, nil

	case HTTPAuthType_Generic:
		return m.genericCredentialManager.Authenticate(ctx, params.Request, params.GenericAuthType)

	case HTTPAuthType_PreDefined:
		return m.preDefinedCredentialManager.Authenticate(ctx, params.Request)

	default:
		return nil, errors.New("invalid auth type")
	}
}

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

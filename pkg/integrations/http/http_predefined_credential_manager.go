package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type PreDefinedCredentialManager interface {
	Authenticate(ctx context.Context, req *http.Request) (*http.Request, error)
}

type preDefinedCredentialManager struct {
	executorCredentialManager domain.ExecutorCredentialManager
	oauthGetter               domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	credentialID              string
}

type PreDefinedCredentialManagerDependencies struct {
	ExecutorCredentialManager domain.ExecutorCredentialManager
	CredentialID              string
}

func NewPreDefinedCredentialManager(deps PreDefinedCredentialManagerDependencies) PreDefinedCredentialManager {
	return &preDefinedCredentialManager{
		executorCredentialManager: deps.ExecutorCredentialManager,
		credentialID:              deps.CredentialID,
	}
}

func (m *preDefinedCredentialManager) Authenticate(ctx context.Context, req *http.Request) (*http.Request, error) {
	if m.credentialID == "" {
		return nil, errors.New("credential is required for pre-defined authentication")
	}

	cred, err := m.executorCredentialManager.GetFullCredential(ctx, m.credentialID)
	if err != nil {
		return nil, err
	}

	switch cred.Type {
	case domain.CredentialTypeOAuth, domain.CredentialTypeOAuthWithParams:
		oauthData, err := m.oauthGetter.GetDecryptedCredential(ctx, m.credentialID)
		if err != nil {
			return nil, fmt.Errorf("pre-defined oauth credential: %w", err)
		}
		if oauthData.AccessToken == "" {
			return nil, errors.New("oauth access token is empty")
		}
		req.Header.Set("Authorization", "Bearer "+oauthData.AccessToken)
		return req, nil

	case domain.CredentialTypeDefault:
		if cred.DecryptedPayload == nil {
			return nil, errors.New("credential payload is empty")
		}
		return applyPreDefinedDefaultCredential(req, cred.DecryptedPayload)

	default:
		return nil, fmt.Errorf("unsupported credential type for HTTP pre-defined auth: %s", cred.Type)
	}
}

func applyPreDefinedDefaultCredential(req *http.Request, payload map[string]any) (*http.Request, error) {
	if u, ok := payloadString(payload, "username"); ok {
		if p, ok := payloadString(payload, "password"); ok {
			req.SetBasicAuth(u, p)
			return req, nil
		}
	}

	for _, key := range []string{"access_token", "token", "api_key", "secret_key", "bearer"} {
		if v, ok := payloadString(payload, key); ok && v != "" {
			req.Header.Set("Authorization", "Bearer "+v)
			return req, nil
		}
	}

	return nil, errors.New("could not derive HTTP authorization from pre-defined credential payload")
}

func payloadString(m map[string]any, k string) (string, bool) {
	v, ok := m[k]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

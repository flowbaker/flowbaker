package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type genericCredentialManager struct {
	executorCredentialManager domain.ExecutorCredentialManager
	credentialID              string
}

type GenericCredentialManager interface {
	Authenticate(ctx context.Context, req *http.Request, genericAuthType HTTPGenericAuthType) (*http.Request, error)
}

type GenericCredentialManagerDependencies struct {
	ExecutorCredentialManager domain.ExecutorCredentialManager
	CredentialID              string
}

func NewGenericCredentialManager(deps GenericCredentialManagerDependencies) GenericCredentialManager {
	return &genericCredentialManager{
		executorCredentialManager: deps.ExecutorCredentialManager,
		credentialID:              deps.CredentialID,
	}
}

type HTTPGenericAuthType string

const (
	HTTPGenericAuthType_Basic  HTTPGenericAuthType = "basic"
	HTTPGenericAuthType_Bearer HTTPGenericAuthType = "bearer"
	HTTPGenericAuthType_Digest HTTPGenericAuthType = "digest"
	HTTPGenericAuthType_Header HTTPGenericAuthType = "header"
	HTTPGenericAuthType_Query  HTTPGenericAuthType = "query"
	HTTPGenericAuthType_JSON   HTTPGenericAuthType = "json"
)

type GenericAuthCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type GenericBearerCredential struct {
	Token string `json:"bearer_token"`
}

type GenericHeaderCredential struct {
	Key   string `json:"header_auth_key"`
	Value string `json:"header_auth_value"`
}

type GenericQueryCredential struct {
	Key   string `json:"query_auth_key"`
	Value string `json:"query_auth_value"`
}

type GenericJSONCredential struct {
	Payload JSONBody `json:"custom_json_payload"`
}

type JSONBody map[string]any

func (m *genericCredentialManager) Authenticate(ctx context.Context, req *http.Request, genericAuthType HTTPGenericAuthType) (*http.Request, error) {
	if m.credentialID == "" {
		return nil, errors.New("credential is required for generic authentication")
	}
	rawCredential, err := m.executorCredentialManager.GetFullCredential(ctx, m.credentialID)
	if err != nil {
		return nil, err
	}

	switch genericAuthType {
	case HTTPGenericAuthType_Basic:
		return m.Basic(req, rawCredential)

	case HTTPGenericAuthType_Bearer:
		return m.Bearer(req, rawCredential)

	// TODO: Implement digest authentication or remove this case
	// case HTTPGenericAuthType_Digest:
	// 	return m.Digest(req, rawCredential)

	case HTTPGenericAuthType_Header:
		return m.Header(req, rawCredential)

	case HTTPGenericAuthType_Query:
		return m.Query(req, rawCredential)

	//  implement this case or remove it
	// case HTTPGenericAuthType_JSON:
	// 	return m.JSON(req, rawCredential)

	default:
		return nil, errors.New("invalid generic auth type")
	}
}

func (m *genericCredentialManager) Basic(req *http.Request, rawCredential domain.Credential) (*http.Request, error) {
	credential := GenericAuthCredential{}
	err := m.readCredential(rawCredential, &credential)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(credential.Username, credential.Password)
	return req, nil
}

func (m *genericCredentialManager) Bearer(req *http.Request, rawCredential domain.Credential) (*http.Request, error) {
	credential := GenericBearerCredential{}
	err := m.readCredential(rawCredential, &credential)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+credential.Token)
	return req, nil
}

func (m *genericCredentialManager) Digest(req *http.Request, _ domain.Credential) (*http.Request, error) {
	return nil, errors.New("HTTP digest authentication is not supported")
}

func (m *genericCredentialManager) Header(req *http.Request, rawCredential domain.Credential) (*http.Request, error) {
	credential := GenericHeaderCredential{}
	err := m.readCredential(rawCredential, &credential)
	if err != nil {
		return nil, err
	}

	if credential.Key == "" {
		return nil, errors.New("header key is required for header auth")
	}

	req.Header.Set(credential.Key, credential.Value)
	return req, nil
}

func (m *genericCredentialManager) Query(req *http.Request, rawCredential domain.Credential) (*http.Request, error) {
	credential := GenericQueryCredential{}
	err := m.readCredential(rawCredential, &credential)
	if err != nil {
		return nil, err
	}

	if credential.Key == "" {
		return nil, errors.New("query key is required for query auth")
	}

	query := req.URL.Query()
	query.Set(credential.Key, credential.Value)
	req.URL.RawQuery = query.Encode()

	return req, nil
}

func (m *genericCredentialManager) JSON(req *http.Request, rawCredential domain.Credential) (*http.Request, error) {
	payload := GenericJSONCredential{}
	err := m.readCredential(rawCredential, &payload)
	if err != nil {
		return nil, err
	}

	requestBody := JSONBody{}
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		_ = req.Body.Close()

		if len(bytes.TrimSpace(bodyBytes)) > 0 {
			if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
				return nil, errors.New("request body must be a JSON object for JSON auth")
			}
		}
	}

	for key, value := range payload.Payload {
		if _, exists := requestBody[key]; exists {
			continue
		}

		requestBody[key] = value
	}

	mergedBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(mergedBody))
	req.ContentLength = int64(len(mergedBody))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(mergedBody)), nil
	}

	return req, nil
}

func (m *genericCredentialManager) readCredential(rawCredential domain.Credential, target any) error {
	jsonPayload, err := json.Marshal(rawCredential.DecryptedPayload)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonPayload, target)
}

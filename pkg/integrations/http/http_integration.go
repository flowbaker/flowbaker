package http

import (
	"context"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type HTTPIntegrationCreator struct {
	integrationSelector    domain.IntegrationSelector
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
	credentialManager      domain.ExecutorCredentialManager
}

func NewHTTPIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &HTTPIntegrationCreator{
		integrationSelector:    deps.IntegrationSelector,
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
		credentialManager:      deps.ExecutorCredentialManager,
	}
}

type HTTPIntegrationDependencies struct {
	IntegrationSelector       domain.IntegrationSelector
	Binder                    domain.IntegrationParameterBinder
	ExecutorStorageManager    domain.ExecutorStorageManager
	ExecutorCredentialManager domain.ExecutorCredentialManager
	WorkspaceID               string
	CredentialID              string
}

type HTTPIntegration struct {
	integrationSelector    domain.IntegrationSelector
	executorStorageManager domain.ExecutorStorageManager
	httpActionManager      HTTPActionManager
	workspaceID            string
	actionManager          *domain.IntegrationActionManager
	credentialID           string
}

func (c *HTTPIntegrationCreator) CreateIntegration(ctx context.Context, params domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewHTTPIntegration(HTTPIntegrationDependencies{
		IntegrationSelector:       c.integrationSelector,
		Binder:                    c.binder,
		ExecutorStorageManager:    c.executorStorageManager,
		ExecutorCredentialManager: c.credentialManager,
		WorkspaceID:               params.WorkspaceID,
		CredentialID:              params.CredentialID,
	})
}

func NewHTTPIntegration(deps HTTPIntegrationDependencies) (*HTTPIntegration, error) {
	integration := &HTTPIntegration{
		integrationSelector:    deps.IntegrationSelector,
		executorStorageManager: deps.ExecutorStorageManager,
		workspaceID:            deps.WorkspaceID,
		credentialID:           deps.CredentialID,
	}

	requestBodyManager := NewRequestBodyManager(RequestBodyManagerDependencies{
		ExecutorStorageManager: deps.ExecutorStorageManager,
		WorkspaceID:            deps.WorkspaceID,
	})

	responseBodyManager := NewResponseBodyManager(ResponseBodyManagerDependencies{
		ExecutorStorageManager: deps.ExecutorStorageManager,
		WorkspaceID:            deps.WorkspaceID,
	})

	credentialManager := NewCredentialManager(CredentialManagerDependencies{
		ExecutorCredentialManager: deps.ExecutorCredentialManager,
		CredentialID:              deps.CredentialID,
	})

	integration.httpActionManager = NewHttpActionManager(HTTPActionManagerDependencies{
		Binder:              deps.Binder,
		CredentialManager:   credentialManager,
		RequestBodyManager:  requestBodyManager,
		ResponseBodyManager: responseBodyManager,
	})

	actionManager := domain.NewIntegrationActionManager()
	actionManager.
		AddPerItem(IntegrationActionType_Post, integration.httpActionManager.Request(HTTPMethod_Post)).
		AddPerItem(IntegrationActionType_Put, integration.httpActionManager.Request(HTTPMethod_Put)).
		AddPerItem(IntegrationActionType_Delete, integration.httpActionManager.Request(HTTPMethod_Delete)).
		AddPerItem(IntegrationActionType_Patch, integration.httpActionManager.Request(HTTPMethod_Patch)).
		AddPerItem(IntegrationActionType_Get, integration.httpActionManager.Request(HTTPMethod_Get))

	integration.actionManager = actionManager

	return integration, nil
}

func (i *HTTPIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.RunPerItem(ctx, params.ActionType, params)
}

type HTTPRequestParams struct {
	BodyType    HTTPBodyType      `json:"body_type"`
	Body        any               `json:"body"`
	URL         string            `json:"url"`
	Headers     []HTTPHeaderParam `json:"headers"`
	QueryParams []HTTPQueryParam  `json:"query_params"`
}

type HTTPBodyType string

const (
	HTTPBodyType_Text                    HTTPBodyType = "text/plain"
	HTTPBodyType_JSON                    HTTPBodyType = "application/json"
	HTTPBodyType_URLEncodedFormData      HTTPBodyType = "application/x-www-form-urlencoded"
	HTTPBodyType_MultiPartFormData       HTTPBodyType = "multipart/form-data"
	HTTPBodyType_Application_OctetStream HTTPBodyType = "application/octet-stream"
)

type HTTPResponse struct {
	StatusCode int         `json:"status_code"`
	Status     string      `json:"status"`
	Headers    http.Header `json:"headers"`
	Body       any         `json:"body"`
}

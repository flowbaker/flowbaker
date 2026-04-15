package http

import (
	"context"
	"io"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type HTTPIntegrationCreator struct {
	integrationSelector    domain.IntegrationSelector
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
}

func NewHTTPIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &HTTPIntegrationCreator{
		integrationSelector:    deps.IntegrationSelector,
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}
}

type HTTPIntegrationDependencies struct {
	IntegrationSelector    domain.IntegrationSelector
	Binder                 domain.IntegrationParameterBinder
	ExecutorStorageManager domain.ExecutorStorageManager
	WorkspaceID            string
}

type HTTPIntegration struct {
	integrationSelector    domain.IntegrationSelector
	executorStorageManager domain.ExecutorStorageManager
	requestBodyManager     RequestBodyManager
	responseBodyManager    ResponseBodyManager
	workspaceID            string
	actionManager          *domain.IntegrationActionManager
	binder                 domain.IntegrationParameterBinder
	client                 *http.Client
}

func (c *HTTPIntegrationCreator) CreateIntegration(ctx context.Context, params domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewHTTPIntegration(HTTPIntegrationDependencies{
		IntegrationSelector:    c.integrationSelector,
		Binder:                 c.binder,
		ExecutorStorageManager: c.executorStorageManager,
		WorkspaceID:            params.WorkspaceID,
	})
}

func NewHTTPIntegration(deps HTTPIntegrationDependencies) (*HTTPIntegration, error) {
	integration := &HTTPIntegration{
		integrationSelector:    deps.IntegrationSelector,
		binder:                 deps.Binder,
		executorStorageManager: deps.ExecutorStorageManager,
		requestBodyManager: NewRequestBodyManager(RequestBodyManagerDependencies{
			ExecutorStorageManager: deps.ExecutorStorageManager,
			WorkspaceID:            deps.WorkspaceID,
		}),
		responseBodyManager: NewResponseBodyManager(ResponseBodyManagerDependencies{
			ExecutorStorageManager: deps.ExecutorStorageManager,
			WorkspaceID:            deps.WorkspaceID,
		}),
		workspaceID: deps.WorkspaceID,
		client:      &http.Client{},
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_Post, integration.PostRequest)

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

type HTTPHeaderParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type HTTPQueryParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type HTTPBodyType string

const (
	HTTPBodyType_Text                    HTTPBodyType = "text/plain"
	HTTPBodyType_JSON                    HTTPBodyType = "application/json"
	HTTPBodyType_URLEncodedFormData      HTTPBodyType = "application/x-www-form-urlencoded"
	HTTPBodyType_MultiPartFormData       HTTPBodyType = "multipart/form-data"
	HTTPBodyType_Application_OctetStream HTTPBodyType = "application/octet-stream"
)

func (i *HTTPIntegration) GetRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	return i.request(ctx, params, item, "GET")
}

func (i *HTTPIntegration) PostRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	return i.request(ctx, params, item, "POST")
}

func (i *HTTPIntegration) PutRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	return i.request(ctx, params, item, "PUT")
}

func (i *HTTPIntegration) PatchRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	return i.request(ctx, params, item, "PATCH")
}

func (i *HTTPIntegration) DeleteRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	return i.request(ctx, params, item, "DELETE")
}

type HTTPRequestResponse struct {
	StatusCode int         `json:"status_code"`
	Status     string      `json:"status"`
	Headers    http.Header `json:"headers"`
	Body       any         `json:"body"`
}

func (i *HTTPIntegration) request(ctx context.Context, params domain.IntegrationInput, item domain.Item, method string) (HTTPRequestResponse, error) {
	p := HTTPRequestParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return HTTPRequestResponse{}, err
	}

	var body io.Reader
	contentType := string(p.BodyType)

	if method != "GET" {
		bodyResult, err := i.requestBodyManager.Create(ctx, CreateRequestBodyParams{
			Body:     p.Body,
			BodyType: p.BodyType,
		})
		if err != nil {
			return HTTPRequestResponse{}, err
		}

		body = bodyResult.Reader
	} else {
		body = nil
	}

	req, err := http.NewRequest(method, p.URL, body)
	if err != nil {
		return HTTPRequestResponse{}, err
	}

	i.setRequestHeaders(req, contentType, p.Headers)
	i.setRequestQueryParams(req, p.QueryParams)

	response, err := i.client.Do(req)
	if err != nil {
		return HTTPRequestResponse{}, err
	}
	defer response.Body.Close()

	responseBody, err := i.responseBodyManager.Parse(ctx, response)
	if err != nil {
		return HTTPRequestResponse{}, err
	}

	return HTTPRequestResponse{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Headers:    response.Header,
		Body:       responseBody,
	}, nil
}

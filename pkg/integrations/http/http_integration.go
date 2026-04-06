package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
)

type HTTPIntegrationCreator struct {
	credentialGetter          domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	httpCredentialGetter      domain.CredentialGetter[HTTPDecryptionResult]
	binder                    domain.IntegrationParameterBinder
	integrationSelector       domain.IntegrationSelector
	executorStorageManager    domain.ExecutorStorageManager
	executorCredentialManager domain.ExecutorCredentialManager
}

func NewHTTPIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &HTTPIntegrationCreator{
		credentialGetter:          managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
		httpCredentialGetter:      managers.NewExecutorCredentialGetter[HTTPDecryptionResult](deps.ExecutorCredentialManager),
		binder:                    deps.ParameterBinder,
		integrationSelector:       deps.IntegrationSelector,
		executorStorageManager:    deps.ExecutorStorageManager,
		executorCredentialManager: deps.ExecutorCredentialManager,
	}
}

type HTTPIntegrationDependencies struct {
	CredentialGetter          domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	HTTPCredentialGetter      domain.CredentialGetter[HTTPDecryptionResult]
	ParameterBinder           domain.IntegrationParameterBinder
	ExecutorCredentialManager domain.ExecutorCredentialManager
	IntegrationSelector       domain.IntegrationSelector
	CredentialID              string
	WorkspaceID               string
	ExecutorStorageManager    domain.ExecutorStorageManager
}

type HTTPIntegration struct {
	binder                    domain.IntegrationParameterBinder
	credentialGetter          domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	httpCredentialGetter      domain.CredentialGetter[HTTPDecryptionResult]
	integrationSelector       domain.IntegrationSelector
	executionStorageManager   domain.ExecutorStorageManager
	executorCredentialManager domain.ExecutorCredentialManager
	httpClientManager         HTTPClientManager
	client                    *http.Client
	bodyReader                io.Reader
	credentialID              string
	workspaceID               string

	actionManager       *domain.IntegrationActionManager
	requestBodyManager  RequestBodyManager
	responseBodyManager ResponseBodyManager
}

func (c *HTTPIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewHTTPIntegration(HTTPIntegrationDependencies{
		WorkspaceID:               p.WorkspaceID,
		CredentialGetter:          c.credentialGetter,
		ParameterBinder:           c.binder,
		IntegrationSelector:       c.integrationSelector,
		CredentialID:              p.CredentialID,
		ExecutorStorageManager:    c.executorStorageManager,
		HTTPCredentialGetter:      c.httpCredentialGetter,
		ExecutorCredentialManager: c.executorCredentialManager,
	})
}

func NewHTTPIntegration(deps HTTPIntegrationDependencies) (*HTTPIntegration, error) {
	integration := &HTTPIntegration{
		binder:                    deps.ParameterBinder,
		credentialID:              deps.CredentialID,
		credentialGetter:          deps.CredentialGetter,
		httpCredentialGetter:      deps.HTTPCredentialGetter,
		integrationSelector:       deps.IntegrationSelector,
		executionStorageManager:   deps.ExecutorStorageManager,
		workspaceID:               deps.WorkspaceID,
		client:                    &http.Client{},
		bodyReader:                nil,
		executorCredentialManager: deps.ExecutorCredentialManager,
	}

	httpClientManager := NewHTTPClientManager(HTTPClientManagerDependencies{
		HTTPCredentialGetter:      deps.HTTPCredentialGetter,
		IntegrationSelector:       deps.IntegrationSelector,
		ExecutorCredentialManager: deps.ExecutorCredentialManager,
		CredentialID:              deps.CredentialID,
	})

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_Get, integration.GetRequest).
		AddPerItem(IntegrationActionType_Post, integration.PostRequest).
		AddPerItem(IntegrationActionType_Put, integration.PutRequest).
		AddPerItem(IntegrationActionType_Patch, integration.PatchRequest).
		AddPerItem(IntegrationActionType_Delete, integration.DeleteRequest)

	requestBodyManager := NewRequestBodyManager(RequestBodyManagerDependencies{
		ExecutorStorageManager: deps.ExecutorStorageManager,
		WorkspaceID:            deps.WorkspaceID,
	})

	responseBodyManager := NewResponseBodyManager(ResponseBodyManagerDependencies{
		ExecutorStorageManager: deps.ExecutorStorageManager,
		WorkspaceID:            deps.WorkspaceID,
	})

	integration.actionManager = actionManager
	integration.requestBodyManager = requestBodyManager
	integration.responseBodyManager = responseBodyManager
	integration.httpClientManager = httpClientManager

	return integration, nil
}

type HTTPRequestParams struct {
	URL          string       `json:"url"`
	Headers      []Header     `json:"headers"`
	QueryParams  []QueryParam `json:"query_params"`
	HttpAuthType HttpAuthType `json:"http_auth_type"`
	BodyType     HTTPBodyType `json:"body_type"`
	Body         any          `json:"body"`
}

type HTTPAuthType string

const (
	HTTPAuthType_NoCredential HTTPAuthType = "no-credential"
	HTTPAuthType_Generic      HTTPAuthType = "generic"
	HTTPAuthType_PreDefined   HTTPAuthType = "pre-defined"
)

type Header = HTTPHeader

type QueryParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type HttpAuthType string

const (
	HttpAuthType_NoCredential HttpAuthType = "no-credential"
	HttpAuthType_Generic      HttpAuthType = "generic"
	HttpAuthType_PreDefined   HttpAuthType = "pre-defined"
)

type HTTPResponse struct {
	StatusCode int         `json:"status_code"`
	Status     string      `json:"status"`
	Header     http.Header `json:"header"`
	Body       any         `json:"body"`
}

type HTTPRequestFunctionParams struct {
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	Headers     []Header     `json:"headers"`
	QueryParams []QueryParam `json:"query_params"`
	BodyType    string       `json:"body_type"`
	BodyReader  io.Reader    `json:"body_reader"`
	RequestType reqType      `json:"request_type"`
}

type reqType string

const (
	reqTypeGet    reqType = "GET"
	reqTypePost   reqType = "POST"
	reqTypePut    reqType = "PUT"
	reqTypePatch  reqType = "PATCH"
	reqTypeDelete reqType = "DELETE"
)

type setRequestBodyParams = RequestBodyParams

type GetHTTPCredentialClientParams struct {
	AuthType HTTPAuthType `json:"auth_type"`
}

func (i *HTTPIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	executeHttpParams := GetHTTPCredentialClientParams{}

	paramsJSON, err := json.Marshal(params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	err = json.Unmarshal(paramsJSON, &executeHttpParams)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	client, err := i.httpClientManager.GetHTTPClient(ctx, GetHTTPCredentialClientParams{
		AuthType: executeHttpParams.AuthType,
	})
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	i.client = client

	return i.actionManager.RunPerItem(ctx, params.ActionType, params)
}

func (i *HTTPIntegration) ExecuteGet(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	allItems := make([]domain.Item, 0)
	for _, nodeItems := range params.ItemsByInputIndex {
		allItems = append(allItems, nodeItems.Items...)
	}

	outputItems := make([]domain.Item, 0, len(allItems))

	for _, item := range allItems {
		p := HTTPRequestParams{}
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		resp, err := i.httpRequest(ctx, HTTPRequestFunctionParams{
			Method:      "GET",
			URL:         p.URL,
			Headers:     p.Headers,
			QueryParams: p.QueryParams,
			BodyType:    "",
			BodyReader:  nil,
			RequestType: reqTypeGet,
		})
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		resp.Body.Close()

	body, err := i.responseBodyManager.SetResponseBody(ctx, resp)
	if err != nil {
		return nil, err
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       body,
	}

		outputItems = append(outputItems, httpResp)
	}

	return domain.IntegrationOutput{
		ItemsByOutputIndex: domain.NewNodeItemsMap(0, params.NodeID, outputItems),
	}, nil
}

func (i *HTTPIntegration) ExecutePost(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	allItems := make([]domain.Item, 0)
	for _, nodeItems := range params.ItemsByInputIndex {
		allItems = append(allItems, nodeItems.Items...)
	}

	outputItems := make([]domain.Item, 0, len(allItems))

	for _, item := range allItems {
		p := HTTPRequestParams{}
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		bodyReader, headers, err := i.setRequestBody(ctx, setRequestBodyParams{
			Headers:            p.Headers,
			BodyType:           HTTPBodyType(p.BodyType),
			JSONBody:           p.JSONBody,
			TextBody:           p.TextBody,
			MultipartFormData:  p.MultipartFormData,
			URLEncodedFormData: p.URLEncodedFormData,
			File:               p.File,
		})
		if err != nil {
			bodyReader = nil
		}

	resp, err := i.httpRequest(ctx, HTTPRequestFunctionParams{
		Method:      "POST",
		URL:         p.URL,
		Headers:     bodyResult.Headers,
		QueryParams: p.QueryParams,
		BodyType:    string(p.BodyType),
		BodyReader:  bodyResult.Reader,
		RequestType: reqTypePost,
	})
	if err != nil {
		log.Error().Msgf("error executing http request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := i.responseBodyManager.SetResponseBody(ctx, resp)
	if err != nil {
		return nil, err
	}

		httpResp := HTTPResponse{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Header:     resp.Header,
			Body:       body,
		}
		outputItems = append(outputItems, httpResp)
	}

	return domain.IntegrationOutput{
		ItemsByOutputIndex: domain.NewNodeItemsMap(0, params.NodeID, outputItems),
	}, nil
}

func (i *HTTPIntegration) ExecutePut(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	allItems := make([]domain.Item, 0)
	for _, nodeItems := range params.ItemsByInputIndex {
		allItems = append(allItems, nodeItems.Items...)
	}

	outputItems := make([]domain.Item, 0, len(allItems))

	for _, item := range allItems {
		p := HTTPRequestParams{}
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		bodyReader, headers, err := i.setRequestBody(ctx, setRequestBodyParams{
			Headers:            p.Headers,
			BodyType:           HTTPBodyType(p.BodyType),
			JSONBody:           p.JSONBody,
			TextBody:           p.TextBody,
			MultipartFormData:  p.MultipartFormData,
			URLEncodedFormData: p.URLEncodedFormData,
			File:               p.File,
		})
		if err != nil {
			bodyReader = nil
		}

	resp, err := i.httpRequest(ctx, HTTPRequestFunctionParams{
		Method:      "PUT",
		URL:         p.URL,
		Headers:     bodyResult.Headers,
		QueryParams: p.QueryParams,
		BodyType:    string(HTTPBodyType_JSON),
		BodyReader:  bodyResult.Reader,
		RequestType: reqTypePut,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := i.responseBodyManager.SetResponseBody(ctx, resp)
	if err != nil {
		return nil, err
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       body,
	}

		outputItems = append(outputItems, httpResp)
	}

	return domain.IntegrationOutput{
		ItemsByOutputIndex: domain.NewNodeItemsMap(0, params.NodeID, outputItems),
	}, nil
}

func (i *HTTPIntegration) ExecutePatch(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	allItems := make([]domain.Item, 0)
	for _, nodeItems := range params.ItemsByInputIndex {
		allItems = append(allItems, nodeItems.Items...)
	}

	outputItems := make([]domain.Item, 0, len(allItems))

	for _, item := range allItems {
		p := HTTPRequestParams{}
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		bodyReader, headers, err := i.setRequestBody(ctx, setRequestBodyParams{
			Headers:            p.Headers,
			BodyType:           HTTPBodyType(p.BodyType),
			JSONBody:           p.JSONBody,
			TextBody:           p.TextBody,
			MultipartFormData:  p.MultipartFormData,
			URLEncodedFormData: p.URLEncodedFormData,
			File:               p.File,
		})
		if err != nil {
			bodyReader = nil
		}

	resp, err := i.httpRequest(ctx, HTTPRequestFunctionParams{
		Method:      "PATCH",
		URL:         p.URL,
		Headers:     bodyResult.Headers,
		QueryParams: p.QueryParams,
		BodyType:    string(HTTPBodyType_JSON),
		BodyReader:  bodyResult.Reader,
		RequestType: reqTypePatch,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := i.responseBodyManager.SetResponseBody(ctx, resp)
	if err != nil {
		return nil, err
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       body,
	}

		outputItems = append(outputItems, httpResp)
	}

	return domain.IntegrationOutput{
		ItemsByOutputIndex: domain.NewNodeItemsMap(0, params.NodeID, outputItems),
	}, nil
}

func (i *HTTPIntegration) ExecuteDelete(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	allItems := make([]domain.Item, 0)
	for _, nodeItems := range params.ItemsByInputIndex {
		allItems = append(allItems, nodeItems.Items...)
	}

	outputItems := make([]domain.Item, 0, len(allItems))

	for _, item := range allItems {
		p := HTTPRequestParams{}
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		bodyReader, headers, err := i.setRequestBody(ctx, setRequestBodyParams{
			Headers:            p.Headers,
			BodyType:           HTTPBodyType(p.BodyType),
			JSONBody:           p.JSONBody,
			TextBody:           p.TextBody,
			MultipartFormData:  p.MultipartFormData,
			URLEncodedFormData: p.URLEncodedFormData,
			File:               p.File,
		})
		if err != nil {
			bodyReader = nil
		}

	resp, err := i.httpRequest(ctx, HTTPRequestFunctionParams{
		Method:      "DELETE",
		URL:         p.URL,
		Headers:     bodyResult.Headers,
		QueryParams: p.QueryParams,
		BodyType:    string(HTTPBodyType_JSON),
		BodyReader:  bodyResult.Reader,
		RequestType: reqTypeDelete,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := i.responseBodyManager.SetResponseBody(ctx, resp)
	if err != nil {
		return nil, err
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       body,
	}

		outputItems = append(outputItems, httpResp)
	}

	return domain.IntegrationOutput{
		ItemsByOutputIndex: domain.NewNodeItemsMap(0, params.NodeID, outputItems),
	}, nil
}

func (i *HTTPIntegration) httpRequest(ctx context.Context, params HTTPRequestFunctionParams) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, string(params.RequestType), params.URL, params.BodyReader)
	if err != nil {
		log.Error().Msgf("error creating http request: %v", err)
		return nil, err
	}

	for _, header := range params.Headers {
		if header.Key != "" || header.Value != "" {
			req.Header.Add(header.Key, header.Value)
		}
	}

	if params.BodyReader == nil {
		req.Header.Del("Content-Type")
		req.Header.Del("Content-Length")
	}

	if len(params.QueryParams) > 0 {
		q := req.URL.Query()
		for _, queryParam := range params.QueryParams {
			if queryParam.Key != "" || queryParam.Value != "" {
				q.Add(queryParam.Key, queryParam.Value)
			}
		}

		req.URL.RawQuery = q.Encode()
	}

	resp, err := i.client.Do(req)
	if err != nil {
		log.Error().Msgf("error executing http request: %v", err)
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Error().Msgf("HTTP request failed with status %d: %s, body: %s", resp.StatusCode, resp.Status, string(body))
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Msgf("error reading response body: %v", err)
		return nil, err
	}
	resp.Body.Close()

	newResp := &http.Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	return newResp, nil
}


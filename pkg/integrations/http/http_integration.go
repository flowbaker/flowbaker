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
	client                    *http.Client
	bodyReader                io.Reader
	credentialID              string
	workspaceID               string

	actionManager       *domain.IntegrationActionManager
	requestBodyManager  *RequestBodyManager
	responseBodyManager *ResponseBodyManager
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

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_Get, integration.GetRequest).
		AddPerItem(IntegrationActionType_Post, integration.PostRequest).
		AddPerItem(IntegrationActionType_Put, integration.PutRequest).
		AddPerItem(IntegrationActionType_Patch, integration.PatchRequest).
		AddPerItem(IntegrationActionType_Delete, integration.DeleteRequest)

	requestBodyManager := NewHTTPRequestBodyManager().
		AddRequestBodyFunc(HTTPBodyType_JSON, integration.buildJSONRequestBody).
		AddRequestBodyFunc(HTTPBodyType_Text, integration.buildTextRequestBody).
		AddRequestBodyFunc(HTTPBodyType_URLEncodedFormData, integration.buildURLEncodedRequestBody).
		AddRequestBodyFunc(HTTPBodyType_MultipartFormData, integration.buildMultipartRequestBody).
		AddRequestBodyFunc(HTTPBodyType_File, integration.buildFileRequestBody)

	responseBodyManager := NewHTTPResponseBodyManager().
		AddResponseBodyFunc(ContentType_Application_JSON, integration.buildJSONResponseBody).
		AddResponseBodyFunc(ContentType_Text_Plain, integration.buildTextResponseBody).
		AddResponseBodyFunc(ContentType_Application_OctetStream, integration.buildOctetStreamResponseBody).
		AddResponseBodyFunc(ContentType_Application_URLEncodedFormData, integration.buildURLEncodedResponseBody).
		AddResponseBodyFunc(ContentType_Image_PNG, integration.buildImageResponseBody).
		AddResponseBodyFunc(ContentType_Image_JPEG, integration.buildImageResponseBody).
		AddResponseBodyFunc(ContentType_Image_JPG, integration.buildImageResponseBody).
		AddResponseBodyFunc(ContentType_Image_GIF, integration.buildImageResponseBody).
		AddResponseBodyFunc(ContentType_Image_WEBP, integration.buildImageResponseBody)

	integration.actionManager = actionManager
	integration.requestBodyManager = requestBodyManager
	integration.responseBodyManager = responseBodyManager

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

type HTTPBodyType string

const (
	HTTPBodyType_JSON               HTTPBodyType = "json"
	HTTPBodyType_Text               HTTPBodyType = "text"
	HTTPBodyType_MultipartFormData  HTTPBodyType = "multipart_form_data"
	HTTPBodyType_URLEncodedFormData HTTPBodyType = "urlencoded_form_data"
	HTTPBodyType_File               HTTPBodyType = "file"
)

type HTTPAuthType string

const (
	HTTPAuthType_NoCredential HTTPAuthType = "no-credential"
	HTTPAuthType_Generic      HTTPAuthType = "generic"
	HTTPAuthType_PreDefined   HTTPAuthType = "pre-defined"
)

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type QueryParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MultipartFormData struct {
	Key   string          `json:"key"`
	Value domain.FileItem `json:"value"`
}

type URLEncodedFormData struct {
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

type setRequestBodyParams struct {
	Headers  []Header     `json:"headers"`
	BodyType HTTPBodyType `json:"body_type"`
	Body     any          `json:"body"`
}

type setResponseBodyParams struct {
	Response    *http.Response
	Body        []byte
	ContentType *contentType
}

type GetHTTPCredentialClientParams struct {
	AuthType HTTPAuthType `json:"auth_type"`
}

type ContentType string

const (
	ContentType_Application_JSON               ContentType = "application/json"
	ContentType_Text_Plain                     ContentType = "text/plain"
	ContentType_Application_URLEncodedFormData ContentType = "application/x-www-form-urlencoded"
	ContentType_Image_PNG                      ContentType = "image/png"
	ContentType_Image_JPEG                     ContentType = "image/jpeg"
	ContentType_Image_JPG                      ContentType = "image/jpg"
	ContentType_Image_GIF                      ContentType = "image/gif"
	ContentType_Image_WEBP                     ContentType = "image/webp"
	ContentType_Application_OctetStream        ContentType = "application/octet-stream"
)

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

	client, err := i.getHTTPClient(ctx, GetHTTPCredentialClientParams{
		AuthType: executeHttpParams.AuthType,
	})
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	i.client = client

	return i.actionManager.RunPerItem(ctx, params.ActionType, params)
}

func (i *HTTPIntegration) GetRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := HTTPRequestParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// For GET requests, we don't want response body
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
		return nil, err
	}
	defer resp.Body.Close()

	body, err := i.setResponseBody(ctx, resp)
	if err != nil {
		return nil, err
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       body,
	}

	return httpResp, nil
}

func (i *HTTPIntegration) PostRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := HTTPRequestParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	bodyReader, headers, err := i.setRequestBody(ctx, setRequestBodyParams{
		Headers:  p.Headers,
		BodyType: HTTPBodyType(p.BodyType),
		Body:     p.Body,
	})
	if err != nil {
		bodyReader = nil
	}

	resp, err := i.httpRequest(ctx, HTTPRequestFunctionParams{
		Method:      "POST",
		URL:         p.URL,
		Headers:     headers,
		QueryParams: p.QueryParams,
		BodyType:    string(p.BodyType),
		BodyReader:  bodyReader,
		RequestType: reqTypePost,
	})
	if err != nil {
		log.Error().Msgf("error executing http request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := i.setResponseBody(ctx, resp)
	if err != nil {
		return nil, err
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       body,
	}

	return httpResp, nil
}

func (i *HTTPIntegration) PutRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := HTTPRequestParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	bodyReader, headers, err := i.setRequestBody(ctx, setRequestBodyParams{
		Headers:  p.Headers,
		BodyType: HTTPBodyType(p.BodyType),
		Body:     p.Body,
	})
	if err != nil {
		bodyReader = nil
	}

	resp, err := i.httpRequest(ctx, HTTPRequestFunctionParams{
		Method:      "PUT",
		URL:         p.URL,
		Headers:     headers,
		QueryParams: p.QueryParams,
		BodyType:    string(HTTPBodyType_JSON),
		BodyReader:  bodyReader,
		RequestType: reqTypePut,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := i.setResponseBody(ctx, resp)
	if err != nil {
		return nil, err
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       body,
	}

	return httpResp, nil
}

func (i *HTTPIntegration) PatchRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := HTTPRequestParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	bodyReader, headers, err := i.setRequestBody(ctx, setRequestBodyParams{
		Headers:  p.Headers,
		BodyType: HTTPBodyType(p.BodyType),
		Body:     p.Body,
	})
	if err != nil {
		bodyReader = nil
	}

	resp, err := i.httpRequest(ctx, HTTPRequestFunctionParams{
		Method:      "PATCH",
		URL:         p.URL,
		Headers:     headers,
		QueryParams: p.QueryParams,
		BodyType:    string(HTTPBodyType_JSON),
		BodyReader:  bodyReader,
		RequestType: reqTypePatch,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := i.setResponseBody(ctx, resp)
	if err != nil {
		return nil, err
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       body,
	}

	return httpResp, nil
}

func (i *HTTPIntegration) DeleteRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := HTTPRequestParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	bodyReader, headers, err := i.setRequestBody(ctx, setRequestBodyParams{
		Headers:  p.Headers,
		BodyType: HTTPBodyType(p.BodyType),
		Body:     p.Body,
	})
	if err != nil {
		bodyReader = nil
	}

	resp, err := i.httpRequest(ctx, HTTPRequestFunctionParams{
		Method:      "DELETE",
		URL:         p.URL,
		Headers:     headers,
		QueryParams: p.QueryParams,
		BodyType:    string(HTTPBodyType_JSON),
		BodyReader:  bodyReader,
		RequestType: reqTypeDelete,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := i.setResponseBody(ctx, resp)
	if err != nil {
		return nil, err
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       body,
	}

	return httpResp, nil
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

func (i *HTTPIntegration) getHTTPClient(ctx context.Context, p GetHTTPCredentialClientParams) (*http.Client, error) {
	switch p.AuthType {
	case HTTPAuthType_Generic:
		client, err := i.getGenericClient(ctx)
		if err != nil {
			return nil, err
		}
		return client, nil

	case HTTPAuthType_PreDefined:
		credential, err := i.executorCredentialManager.GetFullCredential(ctx, i.credentialID)
		if err != nil {
			return nil, err
		}

		client, err := i.getPredefinedClient(ctx, credential)
		if err != nil {
			return nil, err
		}
		return client, nil

	default:
		client, err := i.getNoCredentialClient()
		if err != nil {
			return nil, err
		}
		return client, nil
	}
}

func (i *HTTPIntegration) getNoCredentialClient() (*http.Client, error) {
	return &http.Client{}, nil
}

func (i *HTTPIntegration) getGenericClient(ctx context.Context) (*http.Client, error) {
	decryptionResult, err := i.httpCredentialGetter.GetDecryptedCredential(ctx, i.credentialID)
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

func (i *HTTPIntegration) getPredefinedClient(ctx context.Context, credentialRaw domain.Credential) (*http.Client, error) {
	switch credentialRaw.Type {
	case domain.CredentialTypeOAuth, domain.CredentialTypeOAuthWithParams:
		client, err := i.getPreDefinedOAuthClient(ctx, credentialRaw.IntegrationType, credentialRaw)
		if err != nil {
			return nil, err
		}

		return client, nil

	case domain.CredentialTypeDefault:
		client, err := i.getPreDefinedDefaultClient(ctx, credentialRaw.IntegrationType, credentialRaw)
		if err != nil {
			return nil, err
		}

		return client, nil
	}

	return nil, fmt.Errorf("unsupported credential type for http client provider: %s", credentialRaw.Type)
}

func (i *HTTPIntegration) getPreDefinedOAuthClient(ctx context.Context, integrationTypeCredential domain.IntegrationType, credential domain.Credential) (*http.Client, error) {
	oauthAccount, err := i.executorCredentialManager.GetOAuthAccount(ctx, credential.OAuthAccountID)
	if err != nil {
		return nil, err
	}

	httpClient, err := i.integrationSelector.SelectHTTPOAuthClientProvider(ctx, domain.SelectIntegrationParams{
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

func (i *HTTPIntegration) getPreDefinedDefaultClient(ctx context.Context, integrationTypeCredential domain.IntegrationType, credentialRaw domain.Credential) (*http.Client, error) {
	httpClient, err := i.integrationSelector.SelectHTTPDefaultClientProvider(ctx, domain.SelectIntegrationParams{
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

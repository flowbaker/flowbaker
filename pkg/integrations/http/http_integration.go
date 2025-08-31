package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

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

	actionFuncs map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error)
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

	actionFuncs := map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error){
		IntegrationActionType_Get:    integration.ExecuteGet,
		IntegrationActionType_Post:   integration.ExecutePost,
		IntegrationActionType_Put:    integration.ExecutePut,
		IntegrationActionType_Patch:  integration.ExecutePatch,
		IntegrationActionType_Delete: integration.ExecuteDelete,
	}

	integration.actionFuncs = actionFuncs

	return integration, nil
}

type HTTPRequestParams struct {
	URL                string               `json:"url"`
	Headers            []Header             `json:"headers"`
	QueryParams        []QueryParam         `json:"query_params"`
	HttpAuthType       HttpAuthType         `json:"http_auth_type"`
	BodyType           HTTPBodyType         `json:"body_type"`
	JSONBody           interface{}          `json:"json_body"`
	TextBody           string               `json:"text_body"`
	MultipartFormData  []MultipartFormData  `json:"multipart_form_data_body"`
	URLEncodedFormData []URLEncodedFormData `json:"urlencoded_form_data_body"`
	File               domain.FileItem      `json:"file_body"`
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
	StatusCode int
	Status     string
	Header     http.Header
	Body       any
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
	Headers            []Header             `json:"headers"`
	BodyType           HTTPBodyType         `json:"body_type"`
	JSONBody           interface{}          `json:"json_body"`
	TextBody           string               `json:"text_body"`
	MultipartFormData  []MultipartFormData  `json:"multipart_form_data_body"`
	URLEncodedFormData []URLEncodedFormData `json:"urlencoded_form_data_body"`
	File               domain.FileItem      `json:"file_body"`
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

	actionFunc, ok := i.actionFuncs[params.ActionType]
	if !ok {
		return domain.IntegrationOutput{}, fmt.Errorf("action not found")
	}

	return actionFunc(ctx, params)
}

func (i *HTTPIntegration) ExecuteGet(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]any, 0, len(allItems))

	for _, item := range allItems {
		p := HTTPRequestParams{}
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		// For GET requests, we don't want to send any body
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

		body, err := i.setResponseBody(ctx, resp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		httpResp := HTTPResponse{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Header:     resp.Header,
			Body:       body,
		}

		outputItems = append(outputItems, httpResp)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (i *HTTPIntegration) ExecutePost(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]any, 0, len(allItems))

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
			Headers:     headers,
			QueryParams: p.QueryParams,
			BodyType:    string(p.BodyType),
			BodyReader:  bodyReader,
			RequestType: reqTypePost,
		})
		if err != nil {
			log.Error().Msgf("error executing http request: %v", err)
			return domain.IntegrationOutput{}, err
		}

		resp.Body.Close()

		body, err := i.setResponseBody(ctx, resp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		httpResp := HTTPResponse{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Header:     resp.Header,
			Body:       body,
		}
		outputItems = append(outputItems, httpResp)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (i *HTTPIntegration) ExecutePut(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]any, 0, len(allItems))

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
			Headers:     headers,
			QueryParams: p.QueryParams,
			BodyType:    string(HTTPBodyType_JSON),
			BodyReader:  bodyReader,
			RequestType: reqTypePut,
		})
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		resp.Body.Close()

		body, err := i.setResponseBody(ctx, resp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		httpResp := HTTPResponse{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Header:     resp.Header,
			Body:       body,
		}

		outputItems = append(outputItems, httpResp)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (i *HTTPIntegration) ExecutePatch(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]any, 0, len(allItems))

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
			Headers:     headers,
			QueryParams: p.QueryParams,
			BodyType:    string(HTTPBodyType_JSON),
			BodyReader:  bodyReader,
			RequestType: reqTypePatch,
		})
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		resp.Body.Close()

		body, err := i.setResponseBody(ctx, resp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		httpResp := HTTPResponse{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Header:     resp.Header,
			Body:       body,
		}

		outputItems = append(outputItems, httpResp)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (i *HTTPIntegration) ExecuteDelete(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]any, 0, len(allItems))

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
			Headers:     headers,
			QueryParams: p.QueryParams,
			BodyType:    string(HTTPBodyType_JSON),
			BodyReader:  bodyReader,
			RequestType: reqTypeDelete,
		})
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		resp.Body.Close()

		body, err := i.setResponseBody(ctx, resp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		httpResp := HTTPResponse{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Header:     resp.Header,
			Body:       body,
		}

		outputItems = append(outputItems, httpResp)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (i *HTTPIntegration) httpRequest(ctx context.Context, params HTTPRequestFunctionParams) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, string(params.RequestType), params.URL, params.BodyReader)
	if err != nil {
		log.Error().Msgf("error creating http request: %v", err)
		return nil, err
	}

	// Add headers
	for _, header := range params.Headers {
		if header.Key != "" || header.Value != "" {
			log.Info().Msgf("setting header: %s: %s", header.Key, header.Value)
			req.Header.Add(header.Key, header.Value)
		}
	}

	if params.BodyReader == nil {
		req.Header.Del("Content-Type")
		req.Header.Del("Content-Length")
	}

	// Add query parameters
	if len(params.QueryParams) > 0 {
		q := req.URL.Query()
		for _, queryParam := range params.QueryParams {
			if queryParam.Key != "" || queryParam.Value != "" {
				q.Add(queryParam.Key, queryParam.Value)
			}
		}

		log.Info().Msgf("query params: %+v", q)
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

	// Read and store the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Msgf("error reading response body: %v", err)
		return nil, err
	}
	resp.Body.Close()

	// Create a new response with the body
	newResp := &http.Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	// log.Info().Msgf("response body: %s", string(body))
	return newResp, nil
}

func (i *HTTPIntegration) setRequestBody(ctx context.Context, p setRequestBodyParams) (io.Reader, []Header, error) {
	switch p.BodyType {
	case HTTPBodyType_JSON:
		data, err := json.Marshal(p.JSONBody)
		if err != nil {
			return nil, p.Headers, err
		}

		if string(data) == "null" || len(bytes.TrimSpace(data)) == 0 {
			return nil, p.Headers, nil
		}

		updatedHeaders := append(p.Headers, Header{
			Key:   "Content-Type",
			Value: string(ContentType_Application_JSON),
		})

		return bytes.NewReader(data), updatedHeaders, nil

	case HTTPBodyType_Text:
		trimmed := strings.TrimSpace(p.TextBody)
		if trimmed == "" {
			return nil, p.Headers, nil
		}

		updatedHeaders := append(p.Headers, Header{
			Key:   "Content-Type",
			Value: string(ContentType_Text_Plain),
		})

		return strings.NewReader(trimmed), updatedHeaders, nil

	case HTTPBodyType_URLEncodedFormData:
		form := url.Values{}
		for _, item := range p.URLEncodedFormData {
			form.Add(item.Key, item.Value)
		}

		encoded := strings.TrimSpace(form.Encode())
		if encoded == "" {
			return nil, p.Headers, nil
		}

		updatedHeaders := append(p.Headers, Header{
			Key:   "Content-Type",
			Value: string(ContentType_Application_URLEncodedFormData),
		})

		return strings.NewReader(encoded), updatedHeaders, nil

	case HTTPBodyType_MultipartFormData:
		if len(p.MultipartFormData) == 0 {
			return nil, p.Headers, nil
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		for _, item := range p.MultipartFormData {
			contentType := item.Value.ContentType
			if contentType == "" {
				contentType = string(ContentType_Application_OctetStream)
			}

			if item.Value.ObjectKey != "" {
				executionFile, err := i.executionStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
					WorkspaceID: i.workspaceID,
					UploadID:    item.Value.FileID,
				})
				if err != nil {
					return nil, p.Headers, fmt.Errorf("failed to get file from storage: %w", err)
				}

				partHeader := make(textproto.MIMEHeader)
				partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, item.Key, item.Value.Name))
				partHeader.Set("Content-Type", contentType)

				part, err := writer.CreatePart(partHeader)
				if err != nil {
					return nil, p.Headers, fmt.Errorf("failed to create form file: %w", err)
				}

				fileBytes, err := io.ReadAll(executionFile.Reader)
				if err != nil {
					return nil, p.Headers, fmt.Errorf("failed to read file: %w", err)
				}

				if _, err := part.Write(fileBytes); err != nil {
					return nil, p.Headers, fmt.Errorf("failed to write file data: %w", err)
				}
			} else {
				log.Info().Msgf("writing form field: %s: %s", item.Key, item.Value.Name)
				if err := writer.WriteField(item.Key, item.Value.Name); err != nil {
					return nil, p.Headers, fmt.Errorf("failed to write form field: %w", err)
				}
			}
		}

		if body.Len() == 0 {
			return nil, p.Headers, nil
		}

		if err := writer.Close(); err != nil {
			return nil, p.Headers, fmt.Errorf("failed to close multipart writer: %w", err)
		}

		updatedHeaders := append(p.Headers, Header{
			Key:   "Content-Type",
			Value: writer.FormDataContentType(),
		})

		return body, updatedHeaders, nil

	case HTTPBodyType_File:
		executionFile, err := i.executionStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
			WorkspaceID: i.workspaceID,
			UploadID:    p.File.FileID,
		})
		if err != nil {
			return nil, p.Headers, fmt.Errorf("failed to get file from storage: %w", err)
		}

		updatedHeaders := append(p.Headers, Header{
			Key:   "Content-Type",
			Value: string(ContentType_Application_OctetStream),
		})

		return executionFile.Reader, updatedHeaders, nil

	default:
		return nil, p.Headers, nil
	}
}

type contentType struct {
	Type     string
	Boundary string
}

func parseContentType(header string) (*contentType, error) {
	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty content type header")
	}

	ct := &contentType{
		Type: strings.TrimSpace(parts[0]),
	}

	// Parse parameters
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "boundary=") {
			ct.Boundary = strings.TrimPrefix(part, "boundary=")
		}
	}

	return ct, nil
}

func (i *HTTPIntegration) setResponseBody(ctx context.Context, resp *http.Response) (any, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	contentTypeHeader := resp.Header.Get("Content-Type")
	if contentTypeHeader == "" {
		return nil, fmt.Errorf("Content-Type header is empty")
	}

	contentType, err := parseContentType(contentTypeHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content type: %w", err)
	}

	switch contentType.Type {
	case string(ContentType_Application_JSON):
		var jsonBody map[string]interface{}
		if err := json.Unmarshal(body, &jsonBody); err == nil {
			return jsonBody, nil
		}

	case string(ContentType_Text_Plain):
		return string(body), nil

	case string(ContentType_Application_OctetStream):
		fileName := resp.Header.Get("Content-Disposition")
		if fileName == "" {
			fileName = "unnamed-image"
		}

		fileName = strings.TrimPrefix(fileName, "attachment; filename=")
		fileName = strings.Trim(fileName, "\"")

		bodyReader := io.NopCloser(bytes.NewReader(body))
		defer bodyReader.Close()

		executionFile, err := i.executionStorageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
			WorkspaceID:  i.workspaceID,
			UploadedBy:   i.workspaceID,
			OriginalName: fileName,
			SizeInBytes:  int64(len(body)),
			ContentType:  string(ContentType_Application_OctetStream),
			Reader:       bodyReader,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to put workflow execution object: %w", err)
		}

		return executionFile, nil
	case string(ContentType_Application_URLEncodedFormData):
		return body, nil

	case string(ContentType_Image_PNG), string(ContentType_Image_JPEG), string(ContentType_Image_JPG), string(ContentType_Image_GIF), string(ContentType_Image_WEBP):
		fileName := resp.Header.Get("Content-Disposition")
		if fileName == "" {
			fileName = "unnamed-image." + strings.TrimPrefix(contentType.Type, "image/")
		}

		fileName = strings.TrimPrefix(fileName, "attachment; filename=")
		fileName = strings.Trim(fileName, "\"")

		executionFile, err := i.executionStorageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
			WorkspaceID:  i.workspaceID,
			UploadedBy:   i.workspaceID,
			OriginalName: fileName,
			SizeInBytes:  int64(len(body)),
			ContentType:  string(ContentType_Application_OctetStream),
			Reader:       io.NopCloser(bytes.NewReader(body)),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to put workflow execution object: %w", err)
		}

		return executionFile, nil
	default:
		return body, nil
	}

	return nil, fmt.Errorf("unsupported content type: %s", contentTypeHeader)
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

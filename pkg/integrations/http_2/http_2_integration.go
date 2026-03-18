package http_2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type HTTP2IntegrationCreator struct {
	integrationSelector domain.IntegrationSelector
	binder              domain.IntegrationParameterBinder
}

func NewHTTP2IntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &HTTP2IntegrationCreator{
		integrationSelector: deps.IntegrationSelector,
		binder:              deps.ParameterBinder,
	}
}

type HTTP2IntegrationDependencies struct {
	IntegrationSelector domain.IntegrationSelector
	Binder              domain.IntegrationParameterBinder
}

type HTTP2Integration struct {
	integrationSelector domain.IntegrationSelector
	actionManager       *domain.IntegrationActionManager
	binder              domain.IntegrationParameterBinder
	client              *http.Client
}

func (c *HTTP2IntegrationCreator) CreateIntegration(ctx context.Context, params domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewHTTP2Integration(HTTP2IntegrationDependencies{
		IntegrationSelector: c.integrationSelector,
		Binder:              c.binder,
	})
}

func NewHTTP2Integration(deps HTTP2IntegrationDependencies) (*HTTP2Integration, error) {
	integration := &HTTP2Integration{
		integrationSelector: deps.IntegrationSelector,
		binder:              deps.Binder,
		client:              &http.Client{},
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_Post, integration.PostRequest)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *HTTP2Integration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
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
	HTTPBodyType_Text               HTTPBodyType = "text/plain"
	HTTPBodyType_JSON               HTTPBodyType = "application/json"
	HTTPBodyType_URLEncodedFormData HTTPBodyType = "application/x-www-form-urlencoded"
	HTTPBodyType_MultipartFormData  HTTPBodyType = "multipart/form-data"
	HTTPBodyType_File               HTTPBodyType = "application/octet-stream"
)

func (i *HTTP2Integration) PostRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := HTTPRequestParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	body, err := i.setRequestBody(SetRequestBodyParams{
		Body:     p.Body,
		BodyType: p.BodyType,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", p.URL, body)
	if err != nil {
		return nil, err
	}

	i.setRequestHeaders(req, p.BodyType, p.Headers)
	i.setRequestQueries(req, p.QueryParams)

	response, err := i.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := i.setResponseBody(response)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status_code": response.StatusCode,
		"status":      response.Status,
		"headers":     response.Header,
		"body":        responseBody,
	}, nil
}

type SetRequestBodyParams struct {
	Body     any
	BodyType HTTPBodyType
}

func (i *HTTP2Integration) setRequestHeaders(req *http.Request, bodyType HTTPBodyType, headers []HTTPHeaderParam) {
	req.Header.Set("Content-Type", string(bodyType))

	for _, header := range headers {
		if header.Key == "" {
			continue
		}

		req.Header.Set(header.Key, header.Value)
	}
}

func (i *HTTP2Integration) setRequestQueries(req *http.Request, queryParams []HTTPQueryParam) {
	if len(queryParams) == 0 {
		return
	}

	query := req.URL.Query()

	for _, queryParam := range queryParams {
		if queryParam.Key == "" {
			continue
		}

		query.Add(queryParam.Key, queryParam.Value)
	}

	req.URL.RawQuery = query.Encode()
}

type URLEncodedParams struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (i *HTTP2Integration) setRequestBody(params SetRequestBodyParams) (io.Reader, error) {
	switch params.BodyType {
	case HTTPBodyType_Text:
		stringBodyText, ok := params.Body.(string)
		if !ok {
			return nil, errors.New("body is not a string value")
		}

		return strings.NewReader(stringBodyText), nil

	case HTTPBodyType_JSON:
		jsonBody, err := json.Marshal(params.Body)
		if err != nil {
			return nil, err
		}

		return bytes.NewReader(jsonBody), nil

	case HTTPBodyType_URLEncodedFormData:
		urlValues := url.Values{}

		jsonBody, err := json.Marshal(params.Body)
		if err != nil {
			return nil, err
		}

		var urlEncodedBody []URLEncodedParams
		err = json.Unmarshal(jsonBody, &urlEncodedBody)
		if err != nil {
			return nil, err
		}

		for _, kv := range urlEncodedBody {
			urlValues.Add(kv.Key, kv.Value)
		}

		return strings.NewReader(urlValues.Encode()), nil

	}

	return nil, fmt.Errorf("invalid body type: %s", params.BodyType)
}

type URLEncodedResponse map[string][]string

func (i *HTTP2Integration) setResponseBody(response *http.Response) (any, error) {
	contentType := response.Header.Get("Content-Type")
	if strings.Contains(contentType, ";") {
		contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}

	switch contentType {
	case string(HTTPBodyType_JSON):
		jsonBodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		var jsonBody any
		err = json.Unmarshal(jsonBodyBytes, &jsonBody)
		if err != nil {
			return nil, err
		}

		return jsonBody, nil

	case string(HTTPBodyType_Text):
		textBody, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		return string(textBody), nil

	case string(HTTPBodyType_URLEncodedFormData):
		responseBodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		urlValues, err := url.ParseQuery(string(responseBodyBytes))
		if err != nil {
			return nil, err
		}

		return URLEncodedResponse(urlValues), nil

	}

	return nil, fmt.Errorf("invalid content type: %s", contentType)
}

package http

import (
	"context"
	"io"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type HTTPActionManager interface {
	Request(method HTTPMethod) domain.ActionFuncPerItem
}

type HTTPActionManagerDependencies struct {
	Binder              domain.IntegrationParameterBinder
	CredentialManager   CredentialManager
	RequestBodyManager  RequestBodyManager
	ResponseBodyManager ResponseBodyManager
	Client              *http.Client
}

type httpActionManager struct {
	binder              domain.IntegrationParameterBinder
	credentialManager   CredentialManager
	requestBodyManager  RequestBodyManager
	responseBodyManager ResponseBodyManager
	client              *http.Client
}

type HTTPMethod string

const (
	HTTPMethod_Get    HTTPMethod = "GET"
	HTTPMethod_Post   HTTPMethod = "POST"
	HTTPMethod_Put    HTTPMethod = "PUT"
	HTTPMethod_Delete HTTPMethod = "DELETE"
	HTTPMethod_Patch  HTTPMethod = "PATCH"
)

func NewHttpActionManager(deps HTTPActionManagerDependencies) HTTPActionManager {
	client := deps.Client
	if client == nil {
		client = &http.Client{}
	}

	return &httpActionManager{
		binder:              deps.Binder,
		credentialManager:   deps.CredentialManager,
		requestBodyManager:  deps.RequestBodyManager,
		responseBodyManager: deps.ResponseBodyManager,
		client:              client,
	}
}

func (h *httpActionManager) Request(method HTTPMethod) domain.ActionFuncPerItem {
	return func(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
		p := HTTPRequestParams{}
		err := h.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return HTTPResponse{}, err
		}

		payload, err := h.credentialManager.GetPayload(ctx)
		if err != nil {
			return HTTPResponse{}, err
		}

		var body io.Reader
		contentType := string(p.BodyType)

		if method != HTTPMethod_Get {
			bodyResult, err := h.requestBodyManager.Create(ctx, CreateRequestBodyParams{
				Body:     p.Body,
				BodyType: p.BodyType,
			})
			if err != nil {
				return HTTPResponse{}, err
			}

			body = bodyResult.Reader
		}

		req, err := http.NewRequest(string(method), p.URL, body)
		if err != nil {
			return HTTPResponse{}, err
		}

		req, err = h.credentialManager.Authenticate(ctx, ApplyCredentialParams{
			AuthType:        payload.AuthType,
			GenericAuthType: payload.GenericAuthType,
			Request:         req,
			RequestParams:   p,
			Credential:      payload.Credential,
		})
		if err != nil {
			return HTTPResponse{}, err
		}

		h.setRequestHeaders(req, contentType, p.Headers)
		h.setRequestQueryParams(req, p.QueryParams)

		response, err := h.client.Do(req)
		if err != nil {
			return HTTPResponse{}, err
		}

		defer response.Body.Close()

		responseBody, err := h.responseBodyManager.Parse(ctx, response)
		if err != nil {
			return HTTPResponse{}, err
		}

		return HTTPResponse{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Headers:    response.Header,
			Body:       responseBody,
		}, nil
	}
}

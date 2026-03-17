package domain

import "context"

type HTTPResponse struct {
	StatusCode int          `json:"status_code"`
	Status     string       `json:"status"`
	Header     []HTTPHeader `json:"header"`
	Body       any          `json:"body"`
}

type ContentType string

type ResponseBodyFunc func(ctx context.Context, p setResponseBodyParams) (any, error)

type setResponseBodyParams struct {
	Response    *HTTPResponse
	Body        any
	ContentType *ContentType
}

type ResponseBodyManager struct {
	responseBodyFuncs map[ContentType]ResponseBodyFunc
}

func NewResponseBodyManager() *ResponseBodyManager {
	return &ResponseBodyManager{
		responseBodyFuncs: make(map[ContentType]ResponseBodyFunc),
	}
}

func (m *ResponseBodyManager) AddResponseBodyFunc(contentType ContentType, responseBodyFunc ResponseBodyFunc) *ResponseBodyManager {
	m.responseBodyFuncs[contentType] = responseBodyFunc
	return m
}

func (m *ResponseBodyManager) GetResponseBodyFunc(contentType ContentType) (ResponseBodyFunc, bool) {
	responseBodyFunc, ok := m.responseBodyFuncs[contentType]
	return responseBodyFunc, ok
}

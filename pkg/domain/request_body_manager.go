package domain

import (
	"context"
	"io"
)

type HTTPHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type HTTPBodyType string

const (
	HTTPBodyType_JSON               HTTPBodyType = "json"
	HTTPBodyType_Text               HTTPBodyType = "text"
	HTTPBodyType_MultipartFormData  HTTPBodyType = "multipart_form_data"
	HTTPBodyType_URLEncodedFormData HTTPBodyType = "urlencoded_form_data"
	HTTPBodyType_File               HTTPBodyType = "file"
)

type requestBodyParams struct {
	Headers  []HTTPHeader `json:"headers"`
	BodyType HTTPBodyType `json:"body_type"`
	Body     any          `json:"body"`
}

type RequestBodyFunc func(ctx context.Context, p requestBodyParams) (io.Reader, []HTTPHeader, error)

type RequestBodyManager struct {
	requestBodyFuncs map[HTTPBodyType]RequestBodyFunc
}

func NewRequestBodyManager() *RequestBodyManager {
	return &RequestBodyManager{
		requestBodyFuncs: make(map[HTTPBodyType]RequestBodyFunc),
	}
}

func (m *RequestBodyManager) AddRequestBodyFunc(bodyType HTTPBodyType, requestBodyFunc RequestBodyFunc) *RequestBodyManager {
	m.requestBodyFuncs[bodyType] = requestBodyFunc
	return m
}

func (m *RequestBodyManager) GetRequestBodyFunc(bodyType HTTPBodyType) (RequestBodyFunc, bool) {
	requestBodyFunc, ok := m.requestBodyFuncs[bodyType]
	return requestBodyFunc, ok
}

package http

import (
	"context"
	"io"
)

type RequestBodyFunc func(ctx context.Context, p setRequestBodyParams) (io.Reader, []Header, error)

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

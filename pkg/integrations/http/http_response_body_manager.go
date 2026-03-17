package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type ResponseBodyFunc func(ctx context.Context, p setResponseBodyParams) (any, error)

type ResponseBodyManager struct {
	responseBodyFuncs map[ContentType]ResponseBodyFunc
}

func NewHTTPResponseBodyManager() *ResponseBodyManager {
	return &ResponseBodyManager{
		responseBodyFuncs: make(map[ContentType]ResponseBodyFunc),
	}
}

func (m *ResponseBodyManager) AddResponseBodyFunc(contentType ContentType, fn ResponseBodyFunc) *ResponseBodyManager {
	m.responseBodyFuncs[contentType] = fn
	return m
}

func (m *ResponseBodyManager) GetResponseBodyFunc(contentType ContentType) (ResponseBodyFunc, bool) {
	fn, ok := m.responseBodyFuncs[contentType]
	return fn, ok
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

	ct, err := parseContentType(contentTypeHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content type: %w", err)
	}

	fn, ok := i.responseBodyManager.GetResponseBodyFunc(ContentType(ct.Type))
	if !ok {
		return i.buildDefaultResponseBody(ctx, setResponseBodyParams{
			Response:    resp,
			Body:        body,
			ContentType: ct,
		})
	}

	return fn(ctx, setResponseBodyParams{
		Response:    resp,
		Body:        body,
		ContentType: ct,
	})
}

func (i *HTTPIntegration) buildJSONResponseBody(ctx context.Context, p setResponseBodyParams) (any, error) {
	_ = ctx

	var jsonBody any
	if err := json.Unmarshal(p.Body, &jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}
	return jsonBody, nil
}

func (i *HTTPIntegration) buildTextResponseBody(ctx context.Context, p setResponseBodyParams) (any, error) {
	_ = ctx
	return string(p.Body), nil
}

func (i *HTTPIntegration) buildOctetStreamResponseBody(ctx context.Context, p setResponseBodyParams) (any, error) {
	fileName := p.Response.Header.Get("Content-Disposition")
	if fileName == "" {
		fileName = "unnamed-image"
	}

	fileName = strings.TrimPrefix(fileName, "attachment; filename=")
	fileName = strings.Trim(fileName, "\"")

	bodyReader := io.NopCloser(bytes.NewReader(p.Body))
	defer bodyReader.Close()

	executionFile, err := i.executionStorageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
		WorkspaceID:  i.workspaceID,
		UploadedBy:   i.workspaceID,
		OriginalName: fileName,
		SizeInBytes:  int64(len(p.Body)),
		ContentType:  string(ContentType_Application_OctetStream),
		Reader:       bodyReader,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to put workflow execution object: %w", err)
	}

	return executionFile, nil
}

func (i *HTTPIntegration) buildURLEncodedResponseBody(ctx context.Context, p setResponseBodyParams) (any, error) {
	_ = ctx
	return p.Body, nil
}

func (i *HTTPIntegration) buildImageResponseBody(ctx context.Context, p setResponseBodyParams) (any, error) {
	fileName := p.Response.Header.Get("Content-Disposition")
	if fileName == "" {
		fileName = "unnamed-image." + strings.TrimPrefix(p.ContentType.Type, "image/")
	}

	fileName = strings.TrimPrefix(fileName, "attachment; filename=")
	fileName = strings.Trim(fileName, "\"")

	executionFile, err := i.executionStorageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
		WorkspaceID:  i.workspaceID,
		UploadedBy:   i.workspaceID,
		OriginalName: fileName,
		SizeInBytes:  int64(len(p.Body)),
		ContentType:  string(ContentType_Application_OctetStream),
		Reader:       io.NopCloser(bytes.NewReader(p.Body)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to put workflow execution object: %w", err)
	}

	return executionFile, nil
}

func (i *HTTPIntegration) buildDefaultResponseBody(ctx context.Context, p setResponseBodyParams) (any, error) {
	_ = ctx
	return string(p.Body), nil
}

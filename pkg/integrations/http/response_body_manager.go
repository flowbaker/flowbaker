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

type ContentType string

type ResponseBodyParams struct {
	Body               []byte
	ContentType        string
	ContentDisposition string
}

type ResponseBodyManager interface {
	SetResponseBody(ctx context.Context, resp *http.Response) (any, error)
}

type ResponseBodyManagerDependencies struct {
	ExecutorStorageManager domain.ExecutorStorageManager
	WorkspaceID            string
}

type responseBodyManager struct {
	storageManager domain.ExecutorStorageManager
	workspaceID    string
}

func NewResponseBodyManager(deps ResponseBodyManagerDependencies) ResponseBodyManager {
	return &responseBodyManager{
		storageManager: deps.ExecutorStorageManager,
		workspaceID:    deps.WorkspaceID,
	}
}

type ResponseContentType struct {
	Type     ContentType
	Boundary string
}

func (m *responseBodyManager) SetResponseBody(ctx context.Context, resp *http.Response) (any, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	ct, err := parseContentType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}

	p := ResponseBodyParams{
		Body:               body,
		ContentType:        string(ct.Type),
		ContentDisposition: resp.Header.Get("Content-Disposition"),
	}

	switch ct.Type {
	case ContentType_Application_JSON:
		return m.JSONResponseBody(ctx, p)
	case ContentType_Text_Plain:
		return m.TextResponseBody(ctx, p)
	case ContentType_Application_URLEncodedFormData:
		return m.URLEncodedResponseBody(ctx, p)
	case ContentType_Application_OctetStream:
		return m.OctetStreamResponseBody(ctx, p)
	case ContentType_Image_PNG, ContentType_Image_JPEG, ContentType_Image_JPG,
		ContentType_Image_GIF, ContentType_Image_WEBP:
		return m.ImageResponseBody(ctx, p)
	default:
		return m.DefaultResponseBody(ctx, p)
	}
}

func (m *responseBodyManager) JSONResponseBody(_ context.Context, p ResponseBodyParams) (any, error) {
	var jsonBody any
	if err := json.Unmarshal(p.Body, &jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}
	return jsonBody, nil
}

func (m *responseBodyManager) TextResponseBody(_ context.Context, p ResponseBodyParams) (any, error) {
	return string(p.Body), nil
}

func (m *responseBodyManager) URLEncodedResponseBody(_ context.Context, p ResponseBodyParams) (any, error) {
	return p.Body, nil
}

func (m *responseBodyManager) OctetStreamResponseBody(ctx context.Context, p ResponseBodyParams) (any, error) {
	fileName := p.ContentDisposition
	if fileName == "" {
		fileName = "unnamed-file"
	}
	fileName = strings.TrimPrefix(fileName, "attachment; filename=")
	fileName = strings.Trim(fileName, "\"")

	bodyReader := io.NopCloser(bytes.NewReader(p.Body))
	defer bodyReader.Close()

	executionFile, err := m.storageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
		WorkspaceID:  m.workspaceID,
		UploadedBy:   m.workspaceID,
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

func (m *responseBodyManager) ImageResponseBody(ctx context.Context, p ResponseBodyParams) (any, error) {
	fileName := p.ContentDisposition
	if fileName == "" {
		fileName = "unnamed-image." + strings.TrimPrefix(p.ContentType, "image/")
	}
	fileName = strings.TrimPrefix(fileName, "attachment; filename=")
	fileName = strings.Trim(fileName, "\"")

	executionFile, err := m.storageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
		WorkspaceID:  m.workspaceID,
		UploadedBy:   m.workspaceID,
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

func (m *responseBodyManager) DefaultResponseBody(_ context.Context, p ResponseBodyParams) (any, error) {
	return string(p.Body), nil
}

func parseContentType(header string) (ResponseContentType, error) {
	if header == "" {
		return ResponseContentType{}, fmt.Errorf("Content-Type header is empty")
	}

	parts := strings.Split(header, ";")
	ct := &ResponseContentType{
		Type: ContentType(strings.TrimSpace(parts[0])),
	}

	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "boundary=") {
			ct.Boundary = strings.TrimPrefix(part, "boundary=")
		}
	}

	return *ct, nil
}

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/rs/zerolog/log"
)

type RequestBodyFunc func(ctx context.Context, p setRequestBodyParams) (io.Reader, []Header, error)

type RequestBodyManager struct {
	requestBodyFuncs map[HTTPBodyType]RequestBodyFunc
}

func NewHTTPRequestBodyManager() *RequestBodyManager {
	return &RequestBodyManager{
		requestBodyFuncs: make(map[HTTPBodyType]RequestBodyFunc),
	}
}

func (m *RequestBodyManager) AddRequestBodyFunc(bodyType HTTPBodyType, fn RequestBodyFunc) *RequestBodyManager {
	m.requestBodyFuncs[bodyType] = fn
	return m
}

func (m *RequestBodyManager) GetRequestBodyFunc(bodyType HTTPBodyType) (RequestBodyFunc, bool) {
	fn, ok := m.requestBodyFuncs[bodyType]
	return fn, ok
}

func (i *HTTPIntegration) setRequestBody(ctx context.Context, p setRequestBodyParams) (io.Reader, []Header, error) {
	fn, ok := i.requestBodyManager.GetRequestBodyFunc(p.BodyType)
	if !ok {
		return nil, p.Headers, nil
	}
	return fn(ctx, p)
}

func (i *HTTPIntegration) buildJSONRequestBody(ctx context.Context, p setRequestBodyParams) (io.Reader, []Header, error) {
	_ = ctx

	var data []byte
	var err error

	if strBody, ok := p.Body.(string); ok {
		var parsed interface{}
		if err := json.Unmarshal([]byte(strBody), &parsed); err == nil {
			data, err = json.Marshal(parsed)
			if err != nil {
				return nil, p.Headers, err
			}
		} else {
			data, err = json.Marshal(p.Body)
			if err != nil {
				return nil, p.Headers, err
			}
		}
	} else {
		data, err = json.Marshal(p.Body)
		if err != nil {
			return nil, p.Headers, err
		}
	}

	if string(data) == "null" || len(bytes.TrimSpace(data)) == 0 {
		return nil, p.Headers, nil
	}

	updatedHeaders := append(p.Headers, Header{
		Key:   "Content-Type",
		Value: string(ContentType_Application_JSON),
	})

	return bytes.NewReader(data), updatedHeaders, nil
}

func (i *HTTPIntegration) buildTextRequestBody(ctx context.Context, p setRequestBodyParams) (io.Reader, []Header, error) {
	_ = ctx

	trimmed := strings.TrimSpace(p.Body.(string))
	if trimmed == "" {
		return nil, p.Headers, nil
	}

	updatedHeaders := append(p.Headers, Header{
		Key:   "Content-Type",
		Value: string(ContentType_Text_Plain),
	})

	return strings.NewReader(trimmed), updatedHeaders, nil
}

func (i *HTTPIntegration) buildURLEncodedRequestBody(ctx context.Context, p setRequestBodyParams) (io.Reader, []Header, error) {
	_ = ctx

	form := url.Values{}
	for _, item := range p.Body.([]URLEncodedFormData) {
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
}

func (i *HTTPIntegration) buildMultipartRequestBody(ctx context.Context, p setRequestBodyParams) (io.Reader, []Header, error) {
	if len(p.Body.([]MultipartFormData)) == 0 {
		return nil, p.Headers, nil
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, item := range p.Body.([]MultipartFormData) {
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
}

func (i *HTTPIntegration) buildFileRequestBody(ctx context.Context, p setRequestBodyParams) (io.Reader, []Header, error) {
	executionFile, err := i.executionStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
		WorkspaceID: i.workspaceID,
		UploadID:    p.Body.(domain.FileItem).FileID,
	})
	if err != nil {
		return nil, p.Headers, fmt.Errorf("failed to get file from storage: %w", err)
	}

	updatedHeaders := append(p.Headers, Header{
		Key:   "Content-Type",
		Value: string(ContentType_Application_OctetStream),
	})

	return executionFile.Reader, updatedHeaders, nil
}

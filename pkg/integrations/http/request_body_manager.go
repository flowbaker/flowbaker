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

type URLEncodedFormData struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MultipartFormData struct {
	Key   string          `json:"key"`
	Value domain.FileItem `json:"value"`
}

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

type RequestBodyParams struct {
	Headers  []HTTPHeader `json:"headers"`
	BodyType HTTPBodyType `json:"body_type"`
	Body     any          `json:"body"`
}

type RequestBodyResult struct {
	Reader  io.Reader
	Headers []HTTPHeader
}

type RequestBodyManager interface {
	SetRequestBody(ctx context.Context, p RequestBodyParams) (RequestBodyResult, error)
}

type RequestBodyManagerDependencies struct {
	ExecutorStorageManager domain.ExecutorStorageManager
	WorkspaceID            string
}

type requestBodyManager struct {
	storageManager domain.ExecutorStorageManager
	workspaceID    string
}

func NewRequestBodyManager(deps RequestBodyManagerDependencies) RequestBodyManager {
	return &requestBodyManager{
		storageManager: deps.ExecutorStorageManager,
		workspaceID:    deps.WorkspaceID,
	}
}

func (m *requestBodyManager) SetRequestBody(ctx context.Context, p RequestBodyParams) (RequestBodyResult, error) {
	switch p.BodyType {
	case HTTPBodyType_JSON:
		return m.JSONRequestBody(ctx, p)
	case HTTPBodyType_Text:
		return m.TextRequestBody(ctx, p)
	case HTTPBodyType_URLEncodedFormData:
		return m.URLEncodedRequestBody(ctx, p)
	case HTTPBodyType_MultipartFormData:
		return m.MultipartFormDataRequestBody(ctx, p)
	case HTTPBodyType_File:
		return m.FileRequestBody(ctx, p)
	default:
		return RequestBodyResult{}, fmt.Errorf("invalid body type: %s", p.BodyType)
	}
}

func (m *requestBodyManager) JSONRequestBody(ctx context.Context, p RequestBodyParams) (RequestBodyResult, error) {
	var data []byte
	var err error

	if strBody, ok := p.Body.(string); ok {
		var parsed interface{}
		if err := json.Unmarshal([]byte(strBody), &parsed); err == nil {
			data, err = json.Marshal(parsed)
			if err != nil {
				return RequestBodyResult{Headers: p.Headers}, err
			}
		} else {
			data, err = json.Marshal(p.Body)
			if err != nil {
				return RequestBodyResult{Headers: p.Headers}, err
			}
		}
	} else {
		data, err = json.Marshal(p.Body)
		if err != nil {
			return RequestBodyResult{Headers: p.Headers}, err
		}
	}

	if string(data) == "null" || len(bytes.TrimSpace(data)) == 0 {
		return RequestBodyResult{Headers: p.Headers}, nil
	}

	headers := append(p.Headers, HTTPHeader{
		Key:   "Content-Type",
		Value: string(ContentType_Application_JSON),
	})

	return RequestBodyResult{Reader: bytes.NewReader(data), Headers: headers}, nil
}

func (m *requestBodyManager) TextRequestBody(_ context.Context, p RequestBodyParams) (RequestBodyResult, error) {
	trimmed := strings.TrimSpace(p.Body.(string))
	if trimmed == "" {
		return RequestBodyResult{Headers: p.Headers}, nil
	}

	headers := append(p.Headers, HTTPHeader{
		Key:   "Content-Type",
		Value: string(ContentType_Text_Plain),
	})

	return RequestBodyResult{Reader: strings.NewReader(trimmed), Headers: headers}, nil
}

func (m *requestBodyManager) URLEncodedRequestBody(_ context.Context, p RequestBodyParams) (RequestBodyResult, error) {
	form := url.Values{}
	for _, item := range p.Body.([]URLEncodedFormData) {
		form.Add(item.Key, item.Value)
	}

	encoded := strings.TrimSpace(form.Encode())
	if encoded == "" {
		return RequestBodyResult{Headers: p.Headers}, nil
	}

	headers := append(p.Headers, HTTPHeader{
		Key:   "Content-Type",
		Value: string(ContentType_Application_URLEncodedFormData),
	})

	return RequestBodyResult{Reader: strings.NewReader(encoded), Headers: headers}, nil
}

func (m *requestBodyManager) MultipartFormDataRequestBody(ctx context.Context, p RequestBodyParams) (RequestBodyResult, error) {
	if len(p.Body.([]MultipartFormData)) == 0 {
		return RequestBodyResult{Headers: p.Headers}, nil
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, item := range p.Body.([]MultipartFormData) {
		ct := item.Value.ContentType
		if ct == "" {
			ct = string(ContentType_Application_OctetStream)
		}

		if item.Value.ObjectKey != "" {
			executionFile, err := m.storageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
				WorkspaceID: m.workspaceID,
				UploadID:    item.Value.FileID,
			})
			if err != nil {
				return RequestBodyResult{Headers: p.Headers}, fmt.Errorf("failed to get file from storage: %w", err)
			}

			partHeader := make(textproto.MIMEHeader)
			partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, item.Key, item.Value.Name))
			partHeader.Set("Content-Type", ct)

			part, err := writer.CreatePart(partHeader)
			if err != nil {
				return RequestBodyResult{Headers: p.Headers}, fmt.Errorf("failed to create form file: %w", err)
			}

			fileBytes, err := io.ReadAll(executionFile.Reader)
			if err != nil {
				return RequestBodyResult{Headers: p.Headers}, fmt.Errorf("failed to read file: %w", err)
			}

			if _, err := part.Write(fileBytes); err != nil {
				return RequestBodyResult{Headers: p.Headers}, fmt.Errorf("failed to write file data: %w", err)
			}
		} else {
			log.Info().Msgf("writing form field: %s: %s", item.Key, item.Value.Name)
			if err := writer.WriteField(item.Key, item.Value.Name); err != nil {
				return RequestBodyResult{Headers: p.Headers}, fmt.Errorf("failed to write form field: %w", err)
			}
		}
	}

	if body.Len() == 0 {
		return RequestBodyResult{Headers: p.Headers}, nil
	}

	if err := writer.Close(); err != nil {
		return RequestBodyResult{Headers: p.Headers}, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	headers := append(p.Headers, HTTPHeader{
		Key:   "Content-Type",
		Value: writer.FormDataContentType(),
	})

	return RequestBodyResult{Reader: body, Headers: headers}, nil
}

func (m *requestBodyManager) FileRequestBody(ctx context.Context, p RequestBodyParams) (RequestBodyResult, error) {
	executionFile, err := m.storageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
		WorkspaceID: m.workspaceID,
		UploadID:    p.Body.(domain.FileItem).FileID,
	})
	if err != nil {
		return RequestBodyResult{Headers: p.Headers}, fmt.Errorf("failed to get file from storage: %w", err)
	}

	headers := append(p.Headers, HTTPHeader{
		Key:   "Content-Type",
		Value: string(ContentType_Application_OctetStream),
	})

	return RequestBodyResult{Reader: executionFile.Reader, Headers: headers}, nil
}

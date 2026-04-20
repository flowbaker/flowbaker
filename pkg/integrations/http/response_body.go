package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

const maxResponseFileSize = 1024 * 1024 * 100 // 100MB

type ResponseBodyManager interface {
	Parse(ctx context.Context, response *http.Response) (any, error)
}

type ResponseBodyManagerDependencies struct {
	ExecutorStorageManager domain.ExecutorStorageManager
	WorkspaceID            string
}

type responseBodyManager struct {
	storageManager domain.ExecutorStorageManager
	workspaceID    string
}

type URLEncodedResponse map[string][]string

type MultipartFormDataResponse struct {
	Files Files `json:"files"`
}

type Files map[string]domain.FileItem

func NewResponseBodyManager(deps ResponseBodyManagerDependencies) ResponseBodyManager {
	return &responseBodyManager{
		storageManager: deps.ExecutorStorageManager,
		workspaceID:    deps.WorkspaceID,
	}
}

func (m *responseBodyManager) Parse(ctx context.Context, response *http.Response) (any, error) {
	rawContentType := response.Header.Get("Content-Type")
	contentType := rawContentType
	if strings.Contains(contentType, ";") {
		contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}

	switch contentType {
	case string(HTTPBodyType_JSON):
		return m.JSON(ctx, response)

	case string(HTTPBodyType_Text):
		return m.Text(ctx, response)

	case string(HTTPBodyType_URLEncodedFormData):
		return m.URLEncodedFormData(ctx, response)

	case string(HTTPBodyType_MultiPartFormData):
		return m.MultipartFormData(ctx, response, rawContentType)

	case string(HTTPBodyType_Application_OctetStream):
		return m.ApplicationOctetStream(ctx, response)

	default:
		return nil, errors.New("invalid content type")
	}
}

func (m *responseBodyManager) JSON(ctx context.Context, response *http.Response) (any, error) {
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
}

func (m *responseBodyManager) Text(ctx context.Context, response *http.Response) (any, error) {
	textBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return string(textBody), nil
}

func (m *responseBodyManager) URLEncodedFormData(ctx context.Context, response *http.Response) (any, error) {
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

func (m *responseBodyManager) MultipartFormData(ctx context.Context, response *http.Response, rawContentType string) (any, error) {
	boundary := ""
	for _, part := range strings.Split(rawContentType, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "boundary=") {
			boundary = strings.TrimPrefix(part, "boundary=")
			break
		}
	}

	if boundary == "" {
		return nil, errors.New("multipart response missing boundary")
	}

	multipartReader := multipart.NewReader(response.Body, boundary)
	files := Files{}

	for {
		part, err := multipartReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		fileName := part.FileName()
		if fileName == "" {
			fileName = part.FormName()
		}

		filePayload, err := m.resolveFilePayload(filePayloadParams{
			header:   http.Header(part.Header),
			reader:   part,
			fileName: fileName,
		})
		if err != nil {
			return nil, err
		}

		fileItem, err := m.storageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
			WorkspaceID:  m.workspaceID,
			OriginalName: fileName,
			ContentType:  filePayload.contentType,
			Reader:       filePayload.reader,
			SizeInBytes:  filePayload.contentLength,
			UploadedBy:   m.workspaceID,
		})
		if err != nil {
			return nil, err
		}

		files[part.FormName()] = fileItem
	}

	return MultipartFormDataResponse{Files: files}, nil
}

func (m *responseBodyManager) ApplicationOctetStream(ctx context.Context, response *http.Response) (any, error) {
	fileName := response.Header.Get("Content-Disposition")
	if fileName == "" {
		fileName = "unnamed-file"
	}
	fileName = strings.TrimPrefix(fileName, "attachment; filename=")
	fileName = strings.Trim(fileName, "\"")

	filePayload, err := m.resolveFilePayload(filePayloadParams{
		header:   response.Header,
		reader:   response.Body,
		fileName: fileName,
	})
	if err != nil {
		return nil, err
	}

	fileItem, err := m.storageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
		WorkspaceID:  m.workspaceID,
		OriginalName: fileName,
		ContentType:  filePayload.contentType,
		Reader:       filePayload.reader,
		SizeInBytes:  filePayload.contentLength,
		UploadedBy:   m.workspaceID,
	})
	if err != nil {
		return nil, err
	}

	return fileItem, nil
}

type filePayloadParams struct {
	header   http.Header
	reader   io.Reader
	fileName string
}

type filePayload struct {
	contentType   string
	contentLength int64
	reader        io.ReadCloser
}

func (m *responseBodyManager) resolveFilePayload(src filePayloadParams) (filePayload, error) {
	buf, err := io.ReadAll(io.LimitReader(src.reader, maxResponseFileSize+1))
	if err != nil {
		return filePayload{}, fmt.Errorf("failed to read file content: %w", err)
	}

	if int64(len(buf)) > maxResponseFileSize {
		return filePayload{}, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", maxResponseFileSize)
	}

	if src.header.Get("Content-Length") != "" {
		_, err := strconv.ParseInt(src.header.Get("Content-Length"), 10, 64)
		if err != nil {
			return filePayload{}, fmt.Errorf("failed to parse content length: %w", err)
		}
	}

	contentType := resolveContentType(src.header.Get("Content-Type"), src.fileName, buf)

	return filePayload{
		contentType:   contentType,
		contentLength: int64(len(buf)),
		reader:        io.NopCloser(bytes.NewReader(buf)),
	}, nil
}

func resolveContentType(headerContentType, fileName string, data []byte) string {
	mediaType := normalizeMediaType(headerContentType)
	if mediaType != "" && mediaType != "application/octet-stream" {
		return mediaType
	}

	fileExt := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	if fileExt != "" {
		if resolvedFromExt := normalizeMediaType(mime.TypeByExtension("." + fileExt)); resolvedFromExt != "" {
			return resolvedFromExt
		}
	}

	if len(data) > 0 {
		return normalizeMediaType(http.DetectContentType(data))
	}

	return "application/octet-stream"
}

func normalizeMediaType(contentType string) string {
	if contentType == "" {
		return ""
	}

	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}

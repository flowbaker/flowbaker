package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

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

		fileSize, err := io.Copy(io.Discard, part)
		if err != nil {
			return nil, err
		}

		// read the header to get the content type
		headerBytes := make([]byte, 512)
		n, _ := part.Read(headerBytes)
		headerBytes = headerBytes[:n]

		contentType := http.DetectContentType(headerBytes)

		fileItem, err := m.storageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
			WorkspaceID:  m.workspaceID,
			OriginalName: fileName,
			ContentType:  contentType,
			Reader:       part,
			SizeInBytes:  fileSize,
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
	fileSize, err := io.Copy(io.Discard, response.Body)
	if err != nil {
		return nil, err
	}

	fileName := response.Header.Get("Content-Disposition")
	if fileName == "" {
		fileName = "unnamed-file"
	}
	fileName = strings.TrimPrefix(fileName, "attachment; filename=")
	fileName = strings.Trim(fileName, "\"")

	headerBytes := make([]byte, 512)
	n, _ := response.Body.Read(headerBytes)
	headerBytes = headerBytes[:n]
	contentType := http.DetectContentType(headerBytes)

	fileItem, err := m.storageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
		WorkspaceID:  m.workspaceID,
		OriginalName: fileName,
		ContentType:  contentType,
		Reader:       response.Body,
		SizeInBytes:  fileSize,
		UploadedBy:   m.workspaceID,
	})
	if err != nil {
		return nil, err
	}

	return fileItem, nil
}

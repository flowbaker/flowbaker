package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/url"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type RequestBodyManager interface {
	Create(ctx context.Context, params CreateRequestBodyParams) (CreateRequestBodyResult, error)
}

type RequestBodyManagerDependencies struct {
	ExecutorStorageManager domain.ExecutorStorageManager
	WorkspaceID            string
}

type requestBodyManager struct {
	storageManager domain.ExecutorStorageManager
	workspaceID    string
}

type CreateRequestBodyParams struct {
	Body     any
	BodyType HTTPBodyType
}

type CreateRequestBodyResult struct {
	Reader io.Reader
}
type URLEncodedBody []URLEncodedField

type URLEncodedField struct {
	Key    string            `json:"key"`
	Values []URLEncodedValue `json:"values"`
}

type URLEncodedValue struct {
	Value string `json:"value"`
}

type MultipartBody []MultipartBodyField

type MultipartBodyField struct {
	Name string          `json:"name"`
	File domain.FileItem `json:"file"`
}

type OctetStreamBody struct {
	Name string          `json:"name"`
	File domain.FileItem `json:"file"`
}

func NewRequestBodyManager(deps RequestBodyManagerDependencies) RequestBodyManager {
	return &requestBodyManager{
		storageManager: deps.ExecutorStorageManager,
		workspaceID:    deps.WorkspaceID,
	}
}

func (m *requestBodyManager) Create(ctx context.Context, params CreateRequestBodyParams) (CreateRequestBodyResult, error) {
	switch params.BodyType {
	case HTTPBodyType_Text:
		return m.Text(ctx, params)

	case HTTPBodyType_JSON:
		return m.JSON(ctx, params)

	case HTTPBodyType_URLEncodedFormData:
		return m.URLEncodedFormData(ctx, params)

	case HTTPBodyType_MultiPartFormData:
		return m.MultipartFormData(ctx, params)

	case HTTPBodyType_Application_OctetStream:
		return m.ApplicationOctetStream(ctx, params)

	default:
		return CreateRequestBodyResult{}, errors.New("invalid body type")
	}
}

func (m *requestBodyManager) Text(ctx context.Context, params CreateRequestBodyParams) (CreateRequestBodyResult, error) {
	stringBodyText, ok := params.Body.(string)
	if !ok {
		return CreateRequestBodyResult{}, errors.New("body is not a string value")
	}

	return CreateRequestBodyResult{
		Reader: strings.NewReader(stringBodyText),
	}, nil
}

func (m *requestBodyManager) JSON(ctx context.Context, params CreateRequestBodyParams) (CreateRequestBodyResult, error) {
	jsonBody, err := json.Marshal(params.Body)
	if err != nil {
		return CreateRequestBodyResult{}, err
	}

	return CreateRequestBodyResult{
		Reader: bytes.NewReader(jsonBody),
	}, nil
}

func (m *requestBodyManager) URLEncodedFormData(ctx context.Context, params CreateRequestBodyParams) (CreateRequestBodyResult, error) {
	jsonBody, err := json.Marshal(params.Body)
	if err != nil {
		return CreateRequestBodyResult{}, err
	}

	var urlEncodedBody URLEncodedBody
	err = json.Unmarshal(jsonBody, &urlEncodedBody)
	if err != nil {
		return CreateRequestBodyResult{}, err
	}

	urlValues := url.Values{}
	for _, param := range urlEncodedBody {
		if param.Key == "" {
			continue
		}

		for _, valueItem := range param.Values {
			urlValues.Add(param.Key, valueItem.Value)
		}
	}

	return CreateRequestBodyResult{
		Reader: strings.NewReader(urlValues.Encode()),
	}, nil
}

func (m *requestBodyManager) MultipartFormData(ctx context.Context, params CreateRequestBodyParams) (CreateRequestBodyResult, error) {
	jsonBody, err := json.Marshal(params.Body)
	if err != nil {
		return CreateRequestBodyResult{}, err
	}

	var multipartBody MultipartBody
	err = json.Unmarshal(jsonBody, &multipartBody)
	if err != nil {
		return CreateRequestBodyResult{}, err
	}

	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)

	for _, field := range multipartBody {
		executionFile, err := m.storageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
			WorkspaceID: m.workspaceID,
			UploadID:    field.File.FileID,
		})
		if err != nil {
			return CreateRequestBodyResult{}, err
		}

		partWriter, err := writer.CreateFormFile(field.Name, field.File.Name)
		if err != nil {
			return CreateRequestBodyResult{}, err
		}

		if _, err := io.Copy(partWriter, executionFile.Reader); err != nil {
			return CreateRequestBodyResult{}, err
		}
	}

	if err := writer.Close(); err != nil {
		return CreateRequestBodyResult{}, err
	}

	return CreateRequestBodyResult{
		Reader: bodyBuffer,
	}, nil
}

func (m *requestBodyManager) ApplicationOctetStream(ctx context.Context, params CreateRequestBodyParams) (CreateRequestBodyResult, error) {
	jsonBody, err := json.Marshal(params.Body)
	if err != nil {
		return CreateRequestBodyResult{}, err
	}

	var octetStreamBody OctetStreamBody
	err = json.Unmarshal(jsonBody, &octetStreamBody)
	if err != nil {
		return CreateRequestBodyResult{}, err
	}

	executionFile, err := m.storageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
		WorkspaceID: m.workspaceID,
		UploadID:    octetStreamBody.File.FileID,
	})
	if err != nil {
		return CreateRequestBodyResult{}, err
	}

	return CreateRequestBodyResult{
		Reader: executionFile.Reader,
	}, nil
}

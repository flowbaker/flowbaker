package dropbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

const (
	IntegrationActionType_CopyFile     domain.IntegrationActionType = "copy_file"
	IntegrationActionType_DeleteFile   domain.IntegrationActionType = "delete_file"
	IntegrationActionType_DownloadFile domain.IntegrationActionType = "download_file"
	IntegrationActionType_MoveFile     domain.IntegrationActionType = "move_file"
	IntegrationActionType_UploadFile   domain.IntegrationActionType = "upload_file"
	IntegrationActionType_CreateFolder domain.IntegrationActionType = "create_folder"
	IntegrationActionType_DeleteFolder domain.IntegrationActionType = "delete_folder"
	IntegrationActionType_ListFolder   domain.IntegrationActionType = "list_folder"
	IntegrationActionType_MoveFolder   domain.IntegrationActionType = "move_folder"
	IntegrationActionType_CopyFolder   domain.IntegrationActionType = "copy_folder"
	IntegrationActionType_QueryFolder  domain.IntegrationActionType = "query_folder"
)

const (
	DropboxIntegrationPeekable_Folders domain.IntegrationPeekableType = "folders"
	DropboxIntegrationPeekable_Files   domain.IntegrationPeekableType = "files"
)

type DropboxIntegrationCreator struct {
	credentialGetter       domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
}

func NewDropboxIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &DropboxIntegrationCreator{
		credentialGetter:       managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}
}

type DropboxIntegration struct {
	credentialGetter       domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder                 domain.IntegrationParameterBinder
	client                 *DropboxClient
	executorStorageManager domain.ExecutorStorageManager
	workspaceID            string
	credentialID           string

	actionFuncs map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error)
	peekFuncs   map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type DropboxIntegrationDependencies struct {
	WorkspaceID            string
	CredentialID           string
	CredentialGetter       domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	ParameterBinder        domain.IntegrationParameterBinder
	ExecutorStorageManager domain.ExecutorStorageManager
}

func (c *DropboxIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewDropboxIntegration(ctx, DropboxIntegrationDependencies{
		WorkspaceID:            p.WorkspaceID,
		CredentialID:           p.CredentialID,
		CredentialGetter:       c.credentialGetter,
		ParameterBinder:        c.binder,
		ExecutorStorageManager: c.executorStorageManager,
	})
}

type DropboxClient struct {
	httpClient *http.Client
	baseURL    string
	contentURL string
}

func NewDropboxIntegration(ctx context.Context, deps DropboxIntegrationDependencies) (*DropboxIntegration, error) {
	integration := &DropboxIntegration{
		credentialID:           deps.CredentialID,
		workspaceID:            deps.WorkspaceID,
		credentialGetter:       deps.CredentialGetter,
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}

	actionFuncs := map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error){
		IntegrationActionType_UploadFile:   integration.UploadFile,
		IntegrationActionType_DownloadFile: integration.DownloadFile,
		IntegrationActionType_MoveFile:     integration.MoveFile,
		IntegrationActionType_CopyFile:     integration.CopyFile,
		IntegrationActionType_DeleteFile:   integration.DeleteFile,
		IntegrationActionType_CreateFolder: integration.CreateFolder,
		IntegrationActionType_MoveFolder:   integration.MoveFolder,
		IntegrationActionType_CopyFolder:   integration.CopyFolder,
		IntegrationActionType_DeleteFolder: integration.DeleteFolder,
		// IntegrationActionType_QueryFolder:  integration.QueryFolder,
	}

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		DropboxIntegrationPeekable_Folders: integration.PeekFolders,
		DropboxIntegrationPeekable_Files:   integration.PeekFiles,
	}

	integration.actionFuncs = actionFuncs
	integration.peekFuncs = peekFuncs

	if integration.client == nil {
		log.Info().Msgf("Creating dropbox integration with credential ID: %s", deps.CredentialID)

		credential, err := integration.credentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
		if err != nil {
			log.Error().Msgf("Error getting credential for dropbox integration: %v", err)
			return nil, err
		}

		config := oauth2.Config{}
		token := &oauth2.Token{
			AccessToken: credential.AccessToken,
			TokenType:   "Bearer",
		}

		// Create an HTTP client that automatically adds the authorization header
		httpClient := config.Client(context.TODO(), token)

		integration.client = &DropboxClient{
			httpClient: httpClient,
			baseURL:    "https://api.dropbox.com/2",
			contentURL: "https://content.dropboxapi.com/2",
		}

		if integration.client == nil {
			return nil, fmt.Errorf("failed to create dropbox client")
		}

		log.Info().Msgf("Dropbox integration created with client: %v", integration.client)
	}

	return integration, nil
}

func (i *DropboxIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Executing Dropbox integration")

	actionFunc, ok := i.actionFuncs[params.ActionType]
	if !ok {
		return domain.IntegrationOutput{}, fmt.Errorf("action not found")
	}

	return actionFunc(ctx, params)
}

type CopyFileParams struct {
	FromPath string `json:"from_path"`
	ToPath   string `json:"to_path"`
}

type DeleteFileParams struct {
	FilePath string `json:"file_path"`
}

type UploadFileParams struct {
	FilePath string          `json:"file_path"`
	Content  domain.FileItem `json:"file_content"`
}

type DownloadFileParams struct {
	FilePath string `json:"file_path"`
}

type DeleteFolderParams struct {
	FolderPath string `json:"folder_path"`
}

type MoveFileParams struct {
	FromPath string `json:"from_path"`
	ToPath   string `json:"to_path"`
}

type MoveFolderParams struct {
	FromPath string `json:"from_path"`
	ToPath   string `json:"to_path"`
}

type CopyFolderParams struct {
	FromPath string `json:"from_path"`
	ToPath   string `json:"to_path"`
}

type CreateFolderParams struct {
	FolderPath string `json:"folder_path"`
}

type ListFolderParams struct {
	Path                  string `json:"path"`
	Recursive             bool   `json:"recursive"`
	IncludeMediaInfo      bool   `json:"include_media_info"`
	IncludeDeleted        bool   `json:"include_deleted"`
	IncludeMountedFolders bool   `json:"include_mounted_folders"`
}

type DownloadFileResult struct {
	Content  domain.FileItem   `json:"content"`
	Metadata map[string]string `json:"metadata"`
}

func (i *DropboxIntegration) UploadFile(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Uploading file to Dropbox")
	log.Info().Msgf("Settings: %v", params.IntegrationParams)

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)
	for _, item := range allItems {
		log.Info().Msgf("Item: %v", item)

		var p UploadFileParams

		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if p.FilePath == "" {
			p.FilePath = "/"
		}

		executionFile, err := i.executorStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
			WorkspaceID: i.workspaceID,
			UploadID:    p.Content.FileID,
		})
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to get object: %w", err)
		}

		fileName := p.Content.Name
		filePath := path.Join(p.FilePath, fileName)

		uploadSessionID, err := i.startUploadSession(ctx)
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to start upload session: %w", err)
		}

		reader := executionFile.Reader

		var offset int64

		for offset < executionFile.SizeInBytes {
			if ctx.Err() != nil {
				return domain.IntegrationOutput{}, fmt.Errorf("execution cancelled")
			}

			newOffset, err := i.appendUploadSession(ctx, uploadSessionID, offset, reader)
			if err != nil {
				return domain.IntegrationOutput{}, fmt.Errorf("failed to append upload session: %w", err)
			}

			offset = newOffset
		}

		finishUploadSessionRequest := finishUploadSessionRequest{
			Commit: struct {
				Autorename     bool   `json:"autorename"`
				Mode           string `json:"mode"`
				Mute           bool   `json:"mute"`
				Path           string `json:"path"`
				StrictConflict bool   `json:"strict_conflict"`
			}{
				Autorename:     true,
				Mode:           "add",
				Mute:           false,
				Path:           filePath,
				StrictConflict: false,
			},
			Cursor: struct {
				Offset    int64  `json:"offset"`
				SessionID string `json:"session_id"`
			}{Offset: offset, SessionID: uploadSessionID},
		}

		responseItem, err := i.finishUploadSession(ctx, uploadSessionID, finishUploadSessionRequest)
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to finish upload session: %w", err)
		}

		outputItems = append(outputItems, responseItem)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{resultJSON},
	}, nil
}

type StartUploadSessionResponse struct {
	SessionID string `json:"session_id"`
}

func (i *DropboxIntegration) startUploadSession(ctx context.Context) (string, error) {
	uploadSessionStartEndpoint := fmt.Sprintf("%s/files/upload_session/start", i.client.contentURL)

	uploadStartReq, err := http.NewRequestWithContext(ctx, "POST", uploadSessionStartEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create upload start request: %w", err)
	}

	uploadStartReq.Header.Set("Content-Type", "application/octet-stream")

	uploadStartResp, err := i.client.httpClient.Do(uploadStartReq)
	if err != nil {
		return "", fmt.Errorf("failed to execute upload start request: %w", err)
	}
	defer uploadStartResp.Body.Close()

	if uploadStartResp.StatusCode < 200 || uploadStartResp.StatusCode >= 300 {
		return "", fmt.Errorf("upload start failed with status %d", uploadStartResp.StatusCode)
	}

	body, err := io.ReadAll(uploadStartResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var startUploadSessionResponse StartUploadSessionResponse

	if err := json.Unmarshal(body, &startUploadSessionResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return startUploadSessionResponse.SessionID, nil
}

const (
	chunkSize = 5 * 1024 * 1024

	uploadSessionAppendEndpoint = "/files/upload_session/append_v2"
)

type appendUploadSessionRequest struct {
	Cursor appendUploadCursor `json:"cursor"`
}

type appendUploadCursor struct {
	SessionID string `json:"session_id"`
	Offset    int64  `json:"offset"`
}

func (i *DropboxIntegration) appendUploadSession(ctx context.Context, uploadSessionID string, offset int64, reader io.Reader) (int64, error) {
	chunk := make([]byte, chunkSize)
	n, err := reader.Read(chunk)
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("failed to read from reader: %w", err)
	}
	chunk = chunk[:n]

	log.Info().Msgf("Appending upload session: %s", uploadSessionID)
	log.Info().Msgf("Offset: %d", offset)

	cursor := appendUploadSessionRequest{
		Cursor: appendUploadCursor{
			SessionID: uploadSessionID,
			Offset:    offset,
		},
	}

	cursorJSON, err := json.Marshal(cursor)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal cursor: %w", err)
	}

	endpoint := fmt.Sprintf("%s%s", i.client.contentURL, uploadSessionAppendEndpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(chunk))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(cursorJSON))

	resp, err := i.client.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("append failed with status: %d", resp.StatusCode)
	}

	return offset + int64(n), nil
}

type finishUploadSessionRequest struct {
	Commit struct {
		Autorename     bool   `json:"autorename"`
		Mode           string `json:"mode"`
		Mute           bool   `json:"mute"`
		Path           string `json:"path"`
		StrictConflict bool   `json:"strict_conflict"`
	} `json:"commit"`
	Cursor struct {
		Offset    int64  `json:"offset"`
		SessionID string `json:"session_id"`
	} `json:"cursor"`
}

func (i *DropboxIntegration) finishUploadSession(ctx context.Context, uploadSessionID string, request finishUploadSessionRequest) (map[string]interface{}, error) {
	uploadSessionFinishEndpoint := fmt.Sprintf("%s/files/upload_session/finish", i.client.contentURL)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	uploadFinishReq, err := http.NewRequestWithContext(ctx, "POST", uploadSessionFinishEndpoint, bytes.NewReader(requestJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create upload finish request: %w", err)
	}

	uploadFinishReq.Header.Set("Dropbox-API-Arg", string(requestJSON))
	uploadFinishReq.Header.Set("Content-Type", "application/octet-stream")

	uploadFinishResp, err := i.client.httpClient.Do(uploadFinishReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute upload finish request: %w", err)
	}

	body, err := io.ReadAll(uploadFinishResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if uploadFinishResp.StatusCode < 200 || uploadFinishResp.StatusCode >= 300 {
		return nil, fmt.Errorf("upload finish failed with status %d: body: %s", uploadFinishResp.StatusCode, string(body))
	}

	var responseItem map[string]interface{}

	if err := json.Unmarshal(body, &responseItem); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return responseItem, nil
}

type DownloadFileItemResult struct {
	File domain.FileItem `json:"file"`
}

func (i *DropboxIntegration) DownloadFile(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Downloading file from Dropbox")
	log.Info().Msgf("Settings: %v", params.IntegrationParams)

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		log.Info().Msgf("Item: %v", item)

		var p DownloadFileParams
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		log.Info().Msgf("Downloading file from Dropbox: %s", p.FilePath)

		argHeader, err := json.Marshal(map[string]string{"path": p.FilePath})
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to marshal Dropbox-API-Arg: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/files/download", i.client.contentURL), nil)
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Dropbox-API-Arg", string(argHeader))

		resp, err := i.client.httpClient.Do(req)
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to execute request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return domain.IntegrationOutput{}, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to read response body: %w", err)
		}

		log.Info().Msgf("Downloaded %d bytes", len(respBody))

		fileName := path.Base(p.FilePath)

		log.Info().Msgf("Workspace ID: %s", i.workspaceID)

		fileItem, err := i.executorStorageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
			WorkspaceID:  i.workspaceID,
			UploadedBy:   i.workspaceID,
			OriginalName: fileName,
			SizeInBytes:  int64(len(respBody)),
			ContentType:  resp.Header.Get("Content-Type"),
			Reader:       io.NopCloser(bytes.NewReader(respBody)),
		})
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to store file: %w", err)
		}

		outputItems = append(outputItems, DownloadFileItemResult{
			File: fileItem,
		})
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{resultJSON},
	}, nil
}

func (i *DropboxIntegration) MoveFile(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Settings: %v", params.IntegrationParams)

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)
	for _, item := range allItems {
		log.Info().Msgf("Item: %v", item)

		var p MoveFileParams
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		fromFile := path.Base(p.FromPath)
		p.ToPath = path.Join(p.ToPath, fromFile)

		payload := map[string]interface{}{
			"from_path":                p.FromPath,
			"to_path":                  p.ToPath,
			"allow_shared_folder":      false,
			"allow_ownership_transfer": false,
			"autorename":               true,
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		log.Info().Msgf("Move file params: %v", p)

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/files/move_v2", i.client.baseURL), bytes.NewReader(bodyBytes))
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := i.client.httpClient.Do(req)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return domain.IntegrationOutput{}, fmt.Errorf("move file failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		var parsedResp map[string]interface{}
		err = json.Unmarshal(respBody, &parsedResp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, parsedResp)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{resultJSON},
	}, nil
}

func (i *DropboxIntegration) CopyFile(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Copying file in Dropbox")
	log.Info().Msgf("Settings: %v", params.IntegrationParams)

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)
	for _, item := range allItems {
		log.Info().Msgf("Item: %v", item)

		var p CopyFileParams
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		fromFile := path.Base(p.FromPath)
		p.ToPath = path.Join(p.ToPath, fromFile)

		payload := map[string]interface{}{
			"from_path":                p.FromPath,
			"to_path":                  p.ToPath,
			"allow_shared_folder":      false,
			"allow_ownership_transfer": false,
			"autorename":               true,
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/files/copy_v2", i.client.baseURL), bytes.NewReader(bodyBytes))
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := i.client.httpClient.Do(req)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return domain.IntegrationOutput{}, fmt.Errorf("copy file failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		var parsedResp map[string]interface{}
		err = json.Unmarshal(respBody, &parsedResp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, parsedResp)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{resultJSON},
	}, nil
}

func (i *DropboxIntegration) DeleteFile(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Deleting file in Dropbox")
	log.Info().Msgf("Settings: %v", params.IntegrationParams)

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)
	for _, item := range allItems {
		log.Info().Msgf("Item: %v", item)

		var p DeleteFileParams
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		log.Info().Msgf("From path: %s", p.FilePath)

		payload := map[string]interface{}{
			"path": p.FilePath,
		}
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/files/delete_v2", i.client.baseURL), bytes.NewReader(bodyBytes))
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := i.client.httpClient.Do(req)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Error().Msgf("Dropbox API error: status=%d, body=%s", resp.StatusCode, string(respBody))
			return domain.IntegrationOutput{}, fmt.Errorf("response error: %s", string(respBody))
		}

		var parsedResp map[string]interface{}
		err = json.Unmarshal(respBody, &parsedResp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, parsedResp)

	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{resultJSON},
	}, nil
}

func (i *DropboxIntegration) CreateFolder(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Creating folder in Dropbox")
	log.Info().Msgf("Settings: %v", params.IntegrationParams)

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)
	for _, item := range allItems {
		log.Info().Msgf("Item: %v", item)

		var p CreateFolderParams
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		payload := map[string]interface{}{
			"path":       p.FolderPath,
			"autorename": false,
		}
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/files/create_folder_v2", i.client.baseURL), bytes.NewReader(bodyBytes))
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := i.client.httpClient.Do(req)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Error().Msgf("Dropbox API error: status=%d, body=%s", resp.StatusCode, string(respBody))
			return domain.IntegrationOutput{}, fmt.Errorf("create folder failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		var parsedResp map[string]interface{}
		err = json.Unmarshal(respBody, &parsedResp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, parsedResp)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{resultJSON},
	}, nil
}

func (i *DropboxIntegration) MoveFolder(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Moving folder in Dropbox")
	log.Info().Msgf("Settings: %v", params.IntegrationParams)

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)
	for _, item := range allItems {
		log.Info().Msgf("Item: %v", item)

		var p MoveFolderParams

		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		payload := map[string]interface{}{
			"from_path":                p.FromPath,
			"to_path":                  p.ToPath,
			"allow_shared_folder":      false,
			"autorename":               true,
			"allow_ownership_transfer": false,
		}
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/files/move_v2", i.client.baseURL), bytes.NewReader(bodyBytes))
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := i.client.httpClient.Do(req)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Error().Msgf("Dropbox API error: status=%d, body=%s", resp.StatusCode, string(respBody))
			return domain.IntegrationOutput{}, fmt.Errorf("move folder failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		var parsedResp map[string]interface{}
		err = json.Unmarshal(respBody, &parsedResp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, parsedResp)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{resultJSON},
	}, nil
}

func (i *DropboxIntegration) CopyFolder(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Copying folder in Dropbox")
	log.Info().Msgf("Settings: %v", params.IntegrationParams)

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)
	for _, item := range allItems {
		log.Info().Msgf("Item: %v", item)

		var p CopyFolderParams
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		payload := map[string]interface{}{
			"from_path":                p.FromPath,
			"to_path":                  p.ToPath,
			"allow_shared_folder":      false,
			"autorename":               false,
			"allow_ownership_transfer": false,
		}
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/files/copy_v2", i.client.baseURL), bytes.NewReader(bodyBytes))
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := i.client.httpClient.Do(req)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Error().Msgf("Dropbox API error: status=%d, body=%s", resp.StatusCode, string(respBody))
			return domain.IntegrationOutput{}, fmt.Errorf("copy folder failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		var parsedResp map[string]interface{}
		err = json.Unmarshal(respBody, &parsedResp)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, parsedResp)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{resultJSON},
	}, nil
}

func (i *DropboxIntegration) DeleteFolder(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Deleting folder in Dropbox")
	log.Info().Msgf("Settings: %v", params.IntegrationParams)

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)
	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)
	for _, item := range allItems {
		log.Info().Msgf("Item: %v", item)

		var p DeleteFolderParams
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		payload := map[string]interface{}{
			"path": p.FolderPath,
		}
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/files/delete_v2", i.client.baseURL), bytes.NewReader(bodyBytes))
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		// Check for errors in the response
		resp, err := i.client.httpClient.Do(req)
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to execute request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return domain.IntegrationOutput{}, fmt.Errorf("delete folder failed with status %d: %s", resp.StatusCode, string(body))
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to read response body: %w", err)
		}

		log.Info().Msgf("Response: %s", string(respBody))

		var result map[string]interface{}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		outputItems = append(outputItems, result)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{resultJSON},
	}, nil
}

func (i *DropboxIntegration) Peek(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[p.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peekable type not found")
	}

	return peekFunc(ctx, p)
}

type DropboxItem string

const (
	DropboxItemFolder DropboxItem = "folder"
	DropboxItemFile   DropboxItem = "file"
)

func (i *DropboxIntegration) PeekFolders(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 1000)
	cursor := p.GetCursor()

	folders, err := listDropboxItems(ctx, i.client, listDropboxItemsParams{
		Limit:      limit,
		Cursor:     cursor,
		Path:       p.Path,
		ReturnType: DropboxItemFolder,
	})
	if err != nil {
		return domain.PeekResult{}, err
	}

	result := domain.PeekResult{
		Result: folders.Items,
		Pagination: domain.PaginationMetadata{
			Cursor:  folders.Cursor,
			HasMore: folders.HasMore,
		},
	}

	result.SetCursor(folders.Cursor)
	result.SetHasMore(folders.HasMore)

	return result, nil
}

func (i *DropboxIntegration) PeekFiles(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 1000)
	cursor := p.GetCursor()

	files, err := listDropboxItems(ctx, i.client, listDropboxItemsParams{
		Limit:      limit,
		Cursor:     cursor,
		Path:       p.Path,
		ReturnType: DropboxItemFile,
	})
	if err != nil {
		return domain.PeekResult{}, err
	}

	result := domain.PeekResult{
		Result: files.Items,
		Pagination: domain.PaginationMetadata{
			Cursor:  files.Cursor,
			HasMore: files.HasMore,
		},
	}

	result.SetCursor(files.Cursor)
	result.SetHasMore(files.HasMore)

	return result, nil
}

type listDropboxItemsResult struct {
	Items   []domain.PeekResultItem
	Cursor  string
	HasMore bool
}

type listDropboxItemsParams struct {
	Limit      int
	Cursor     string
	Path       string
	ReturnType DropboxItem
}

func listDropboxItems(ctx context.Context, client *DropboxClient, params listDropboxItemsParams) (
	listDropboxItemsResult,
	error,
) {
	var url string
	var requestBody []byte
	var err error

	if params.Cursor == "" {
		url = fmt.Sprintf("%s/files/list_folder", client.baseURL)
		requestBody, err = json.Marshal(map[string]interface{}{
			"path":      params.Path,
			"recursive": true,
			"limit":     params.Limit,
		})

	} else {
		url = fmt.Sprintf("%s/files/list_folder/continue", client.baseURL)
		requestBody, err = json.Marshal(map[string]interface{}{
			"cursor": params.Cursor,
		})

	}

	if err != nil {
		return listDropboxItemsResult{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return listDropboxItemsResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return listDropboxItemsResult{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return listDropboxItemsResult{}, fmt.Errorf("list folder failed with status %d: %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return listDropboxItemsResult{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		Entries []struct {
			Tag         string `json:".tag"`
			PathDisplay string `json:"path_display"`
		} `json:"entries"`
		Cursor  string `json:"cursor"`
		HasMore bool   `json:"has_more"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return listDropboxItemsResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var items []domain.PeekResultItem

	for _, entry := range result.Entries {
		item := domain.PeekResultItem{
			Key:     entry.PathDisplay,
			Value:   entry.PathDisplay,
			Content: entry.PathDisplay,
		}
		switch params.ReturnType {
		case DropboxItemFolder:
			if entry.Tag == "folder" {
				items = append(items, item)
			}
		case DropboxItemFile:
			if entry.Tag == "file" {
				items = append(items, item)
			}
		}
	}

	return listDropboxItemsResult{Items: items, Cursor: result.Cursor, HasMore: result.HasMore}, nil
}

package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
)

const (
	StorageIntegrationPeekable_Files   domain.IntegrationPeekableType = "files"
	StorageIntegrationPeekable_Folders domain.IntegrationPeekableType = "folders"
)

type StorageIntegrationCreator struct {
	executorStorageManager domain.ExecutorStorageManager
	parameterBinder        domain.IntegrationParameterBinder
}

func NewStorageIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &StorageIntegrationCreator{
		executorStorageManager: deps.ExecutorStorageManager,
		parameterBinder:        deps.ParameterBinder,
	}
}

func (c *StorageIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewStorageIntegration(StorageIntegrationDependencies{
		ExecutorStorageManager: c.executorStorageManager,
		ParameterBinder:        c.parameterBinder,
		WorkspaceID:            p.WorkspaceID,
	}), nil
}

type StorageIntegration struct {
	executorStorageManager domain.ExecutorStorageManager
	parameterBinder        domain.IntegrationParameterBinder
	workspaceID            string

	actionManager *domain.IntegrationActionManager
	peekFuncs     map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type StorageIntegrationDependencies struct {
	ExecutorStorageManager domain.ExecutorStorageManager
	ParameterBinder        domain.IntegrationParameterBinder
	WorkspaceID            string
}

func NewStorageIntegration(deps StorageIntegrationDependencies) *StorageIntegration {
	integration := &StorageIntegration{
		executorStorageManager: deps.ExecutorStorageManager,
		parameterBinder:        deps.ParameterBinder,
		workspaceID:            deps.WorkspaceID,
	}

	// Initialize action manager with proper action types
	actionManager := domain.NewIntegrationActionManager().
		AddPerItemMulti(ActionListFiles, integration.ListFiles).
		AddPerItem(ActionGetFile, integration.GetFile).
		AddPerItem(ActionDeleteFile, integration.DeleteFile).
		AddPerItem(ActionPersistFile, integration.PersistFile)

	integration.actionManager = actionManager

	// Initialize peek functions
	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error){
		StorageIntegrationPeekable_Files:   integration.PeekFiles,
		StorageIntegrationPeekable_Folders: integration.PeekFolders,
	}

	integration.peekFuncs = peekFuncs

	return integration
}

func (i *StorageIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

// FileResponse wraps a file item for API responses
type FileResponse struct {
	File domain.FileItem `json:"file"`
}

// ListFiles - List files in workspace with optional filtering
type ListFilesParams struct {
	Limit      *int    `json:"limit,omitempty"`
	FromFileID *string `json:"from_file_id,omitempty"`
	FolderID   *string `json:"folder_id,omitempty"`
}

func (i *StorageIntegration) ListFiles(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := ListFilesParams{}

	err := i.parameterBinder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	// Set default limit if not provided
	limit := 1000
	if p.Limit != nil {
		limit = *p.Limit
	}

	if p.FolderID != nil && *p.FolderID == "" {
		p.FolderID = nil
	}

	// List workspace files
	result, err := i.executorStorageManager.ListWorkspaceFiles(ctx, domain.ListWorkspaceFilesParams{
		WorkspaceID: i.workspaceID,
		FolderID:    p.FolderID,
		Cursor:      "",
		Limit:       limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace files: %w", err)
	}

	// Convert to domain items
	var items []domain.Item
	for _, file := range result.Files {
		// Get FileItem using execution storage manager
		fileItem, err := i.executorStorageManager.GetExecutionFileAsFileItem(ctx, domain.GetExecutionFileParams{
			WorkspaceID: i.workspaceID,
			UploadID:    file.UploadID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get file item for %s: %w", file.UploadID, err)
		}

		// Wrap in FileResponse struct
		fileResponse := FileResponse{
			File: fileItem,
		}

		items = append(items, fileResponse)
	}

	log.Info().
		Str("workspace_id", i.workspaceID).
		Int("file_count", len(items)).
		Msg("Storage integration: Listed files")

	return items, nil
}

// GetFile - Retrieve file content and return as file item
type GetFileParams struct {
	FileID string `json:"file_id"`
}

func (i *StorageIntegration) GetFile(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetFileParams{}

	err := i.parameterBinder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if p.FileID == "" {
		return nil, fmt.Errorf("file_id is required")
	}

	// Get file item using execution storage manager
	fileItem, err := i.executorStorageManager.GetExecutionFileAsFileItem(ctx, domain.GetExecutionFileParams{
		WorkspaceID: i.workspaceID,
		UploadID:    p.FileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file item: %w", err)
	}

	// Create response item with FileResponse wrapper
	responseItem := FileResponse{
		File: fileItem,
	}

	log.Info().
		Str("workspace_id", i.workspaceID).
		Str("file_id", p.FileID).
		Str("file_name", fileItem.Name).
		Msg("Storage integration: Retrieved file")

	return responseItem, nil
}

// DeleteFile - Remove file from storage
type DeleteFileParams struct {
	FileID string `json:"file_id"`
}

func (i *StorageIntegration) DeleteFile(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteFileParams{}

	err := i.parameterBinder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if p.FileID == "" {
		return nil, fmt.Errorf("file_id is required")
	}

	// Delete the file using upload service
	err = i.executorStorageManager.DeleteExecutionFile(ctx, domain.DeleteExecutionFileParams{
		WorkspaceID: i.workspaceID,
		UploadID:    p.FileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}

	// Create response with deletion information
	response := map[string]interface{}{
		"file_id": p.FileID,
		"deleted": true,
	}

	log.Info().
		Str("workspace_id", i.workspaceID).
		Str("file_id", p.FileID).
		Msg("Storage integration: Deleted file")

	return response, nil
}

// PersistFile - Remove expiration date from file to prevent automatic deletion
type PersistFileParams struct {
	File domain.FileItem `json:"file"`
}

func (i *StorageIntegration) PersistFile(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := PersistFileParams{}

	err := i.parameterBinder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if p.File.FileID == "" {
		return nil, fmt.Errorf("file is required")
	}

	// Persist the file using execution storage manager
	err = i.executorStorageManager.PersistExecutionFile(ctx, domain.PersistExecutionFileParams{
		WorkspaceID: i.workspaceID,
		FileID:      p.File.FileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to persist file: %w", err)
	}

	// Create response with persistence information
	response := map[string]interface{}{
		"file_id":   p.File.FileID,
		"file_name": p.File.Name,
		"persisted": true,
		"message":   "File expiration removed - file will not be automatically deleted",
	}

	log.Info().
		Str("workspace_id", i.workspaceID).
		Str("file_id", p.File.FileID).
		Str("file_name", p.File.Name).
		Msg("Storage integration: File persisted successfully")

	return response, nil
}

// Peek - Implementation of IntegrationPeeker interface
func (i *StorageIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found for type: %s", params.PeekableType)
	}

	return peekFunc(ctx, params)
}

// PeekFiles - Browse files in workspace
type PeekFilesParams struct {
	WorkspaceID string  `json:"workspace_id"`
	FolderID    *string `json:"folder_id,omitempty"`
	Limit       int     `json:"limit,omitempty"`
}

func (i *StorageIntegration) PeekFiles(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	var peekParams PeekFilesParams
	if len(params.PayloadJSON) > 0 {
		if err := json.Unmarshal(params.PayloadJSON, &peekParams); err != nil {
			return domain.PeekResult{}, fmt.Errorf("failed to unmarshal peek params: %w", err)
		}
	}

	workspaceID := params.WorkspaceID
	if workspaceID == "" {
		workspaceID = i.workspaceID
	}

	if peekParams.Limit == 0 {
		peekParams.Limit = 20
	}

	result, err := i.executorStorageManager.ListWorkspaceFiles(ctx, domain.ListWorkspaceFilesParams{
		WorkspaceID: workspaceID,
		FolderID:    peekParams.FolderID,
		Cursor:      params.Pagination.Cursor,
		Limit:       peekParams.Limit,
	})
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list workspace files: %w", err)
	}

	// Convert to peek result items
	var items []domain.PeekResultItem
	for _, file := range result.Files {
		items = append(items, domain.PeekResultItem{
			Key:     file.UploadID,
			Value:   file.UploadID,
			Content: fmt.Sprintf("%s (%s)", file.FileName, formatFileSize(file.Size)),
		})
	}

	peekResult := domain.PeekResult{
		Result: items,
		Pagination: domain.PaginationMetadata{
			Cursor:  result.NextCursor,
			HasMore: result.NextCursor != "",
		},
	}
	peekResult.Pagination.Cursor = result.NextCursor
	peekResult.Pagination.NextCursor = result.NextCursor
	peekResult.Pagination.HasMore = result.NextCursor != ""

	return peekResult, nil
}

// PeekFolders - Browse folders in workspace
type PeekFoldersParams struct {
	WorkspaceID    string  `json:"workspace_id"`
	ParentFolderID *string `json:"parent_folder_id,omitempty"`
}

func (i *StorageIntegration) PeekFolders(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	var peekParams PeekFoldersParams
	if len(params.PayloadJSON) > 0 {
		if err := json.Unmarshal(params.PayloadJSON, &peekParams); err != nil {
			return domain.PeekResult{}, fmt.Errorf("failed to unmarshal peek params: %w", err)
		}
	}

	workspaceID := params.WorkspaceID
	if workspaceID == "" {
		workspaceID = i.workspaceID
	}

	result, err := i.executorStorageManager.ListFolders(ctx, domain.ListFoldersParams{
		WorkspaceID:    workspaceID,
		ParentFolderID: peekParams.ParentFolderID,
		IncludeDeleted: false,
		AllFolders:     false,
	})
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list folders: %w", err)
	}

	// Convert to peek result items
	var items []domain.PeekResultItem
	for _, folder := range result.Folders {
		items = append(items, domain.PeekResultItem{
			Key:     folder.ID,
			Value:   folder.ID,
			Content: fmt.Sprintf("%s (%d files)", folder.Name, folder.FileCount),
		})
	}

	peekResult := domain.PeekResult{
		Result: items,
		Pagination: domain.PaginationMetadata{
			HasMore: false,
		},
	}
	peekResult.Pagination.HasMore = false

	return peekResult, nil
}

// Helper function to format file sizes in human-readable format
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

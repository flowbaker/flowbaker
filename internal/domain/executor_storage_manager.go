package domain

import (
	"context"
	"errors"
	"fmt"
	"io"

	"flowbaker/pkg/flowbaker"

	"github.com/rs/xid"
)

var (
	ErrUploadNotFound = errors.New("upload not found")
)

type ExecutionWorkspaceFile struct {
	ID          string
	WorkspaceID string
	Name        string
	SizeInBytes int64
	ContentType string
	UploadedBy  string

	Reader io.ReadCloser
}

type GetExecutionFileParams struct {
	WorkspaceID string
	UploadID    string
}

type PutExecutionFileParams struct {
	WorkspaceID  string
	UploadedBy   string
	OriginalName string  // Optional
	SizeInBytes  int64   // Optional
	ContentType  string  // Optional
	FolderID     *string // Optional - folder to place the file in
	Reader       io.ReadCloser
}

type DeleteExecutionFileParams struct {
	WorkspaceID string
	UploadID    string
}

type PersistExecutionFileParams struct {
	WorkspaceID string
	FileID      string // UploadID of the file to persist
}

type ExecutorStorageManager interface {
	GetExecutionFile(ctx context.Context, params GetExecutionFileParams) (ExecutionWorkspaceFile, error)
	GetExecutionFileAsFileItem(ctx context.Context, params GetExecutionFileParams) (FileItem, error)
	PutExecutionFile(ctx context.Context, params PutExecutionFileParams) (FileItem, error)
	PersistExecutionFile(ctx context.Context, params PersistExecutionFileParams) error

	ListWorkspaceFiles(ctx context.Context, params ListWorkspaceFilesParams) (ListWorkspaceFilesResult, error)
	DeleteExecutionFile(ctx context.Context, params DeleteExecutionFileParams) error
	ListFolders(ctx context.Context, params ListFoldersParams) (ListFoldersResult, error)
}

type executorStorageManager struct {
	client *flowbaker.Client
}

type ExecutorStorageManagerDependencies struct {
	Client *flowbaker.Client
}

func NewExecutorStorageManager(deps ExecutorStorageManagerDependencies) ExecutorStorageManager {
	return &executorStorageManager{
		client: deps.Client,
	}
}

func (s *executorStorageManager) GetExecutionFile(ctx context.Context, params GetExecutionFileParams) (ExecutionWorkspaceFile, error) {
	readerResult, err := s.client.GetFileReader(ctx, &flowbaker.GetFileReaderRequest{
		WorkspaceID: params.WorkspaceID,
		UploadID:    params.UploadID,
	})
	if err != nil {
		return ExecutionWorkspaceFile{}, fmt.Errorf("failed to get file reader: %w", err)
	}

	return ExecutionWorkspaceFile{
		ID:          params.UploadID,
		WorkspaceID: params.WorkspaceID,
		Name:        readerResult.FileName,
		SizeInBytes: readerResult.FileSize,
		ContentType: readerResult.ContentType,
		UploadedBy:  "", // Not available in API response
		Reader:      readerResult.Reader,
	}, nil
}

func (s *executorStorageManager) GetExecutionFileAsFileItem(ctx context.Context, params GetExecutionFileParams) (FileItem, error) {
	fileInfoResp, err := s.client.GetFileInfo(ctx, &flowbaker.GetFileInfoRequest{
		WorkspaceID: params.WorkspaceID,
		UploadID:    params.UploadID,
	})
	if err != nil {
		return FileItem{}, fmt.Errorf("failed to get file info: %w", err)
	}

	if fileInfoResp.Status != "completed" {
		return FileItem{}, ErrUploadNotFound
	}

	return FileItem{
		FileID:      params.UploadID,
		WorkspaceID: params.WorkspaceID,
		ObjectKey:   fileInfoResp.FileName,
		Name:        fileInfoResp.FileName,
		SizeInBytes: fileInfoResp.SizeInBytes,
		ContentType: fileInfoResp.ContentType,
		URL:         "", // Not applicable for executor storage
	}, nil
}

func (s *executorStorageManager) PutExecutionFile(ctx context.Context, params PutExecutionFileParams) (FileItem, error) {
	fileName := params.OriginalName
	if fileName == "" {
		fileName = xid.New().String()
	}

	contentType := params.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	writerResult, err := s.client.CreateFileWriter(ctx, &flowbaker.CreateFileWriterRequest{
		FileName:    fileName,
		ContentType: contentType,
		FolderID:    params.FolderID,
		WorkspaceID: params.WorkspaceID,
		TotalSize:   params.SizeInBytes,
	})
	if err != nil {
		return FileItem{}, fmt.Errorf("failed to create file writer: %w", err)
	}
	defer writerResult.Writer.Close()

	bytesWritten, err := io.Copy(writerResult.Writer, params.Reader)
	if err != nil {
		return FileItem{}, fmt.Errorf("failed to stream file content: %w", err)
	}

	if err := writerResult.Writer.Close(); err != nil {
		return FileItem{}, fmt.Errorf("failed to complete upload: %w", err)
	}

	return FileItem{
		FileID:      writerResult.UploadID,
		WorkspaceID: params.WorkspaceID,
		ObjectKey:   fileName,
		Name:        fileName,
		SizeInBytes: bytesWritten,
		ContentType: contentType,
		URL:         "", // Not applicable for executor storage
	}, nil
}

func (s *executorStorageManager) PersistExecutionFile(ctx context.Context, params PersistExecutionFileParams) error {
	resp, err := s.client.PersistFile(ctx, &flowbaker.PersistFileRequest{
		WorkspaceID: params.WorkspaceID,
		UploadID:    params.FileID,
	})
	if err != nil {
		return fmt.Errorf("failed to persist file: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("persist file failed: %s", resp.Message)
	}

	return nil
}

type ListWorkspaceFilesParams struct {
	WorkspaceID string
	FolderID    *string // nil for root level, specific ID for folder contents
	Cursor      string
	Limit       int
}

type ListWorkspaceFilesResult struct {
	Files      []WorkspaceFile
	NextCursor string
}

func (s *executorStorageManager) ListWorkspaceFiles(ctx context.Context, params ListWorkspaceFilesParams) (ListWorkspaceFilesResult, error) {
	resp, err := s.client.ListWorkspaceFiles(ctx, &flowbaker.ListWorkspaceFilesRequest{
		WorkspaceID: params.WorkspaceID,
		FolderID:    params.FolderID,
		Cursor:      params.Cursor,
		Limit:       params.Limit,
	})
	if err != nil {
		return ListWorkspaceFilesResult{}, fmt.Errorf("failed to list workspace files: %w", err)
	}

	files := make([]WorkspaceFile, len(resp.Files))

	for i, file := range resp.Files {
		files[i] = WorkspaceFile{
			UploadID:    file.UploadID,
			UploadedBy:  file.UploadedBy,
			WorkspaceID: file.WorkspaceID,
			FileName:    file.FileName,
			ContentType: file.ContentType,
			Size:        file.Size,
			UploadedAt:  file.UploadedAt,
			ExpiresAt:   file.ExpiresAt,
			TagIDs:      file.TagIDs,
			FolderID:    file.FolderID,
		}
	}

	return ListWorkspaceFilesResult{
		Files:      files,
		NextCursor: resp.NextCursor,
	}, nil
}

func (s *executorStorageManager) DeleteExecutionFile(ctx context.Context, params DeleteExecutionFileParams) error {
	resp, err := s.client.DeleteFile(ctx, &flowbaker.DeleteFileRequest{
		WorkspaceID: params.WorkspaceID,
		UploadID:    params.UploadID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("delete file failed: %s", resp.Message)
	}

	return nil
}

type ListFoldersParams struct {
	WorkspaceID    string
	ParentFolderID *string
	IncludeDeleted bool
	AllFolders     bool // When true, ignore ParentFolderID and return all folders
}

type ListFoldersResult struct {
	Folders []Folder
}

func (s *executorStorageManager) ListFolders(ctx context.Context, params ListFoldersParams) (ListFoldersResult, error) {
	resp, err := s.client.ListFolders(ctx, &flowbaker.ListFoldersRequest{
		WorkspaceID:    params.WorkspaceID,
		ParentFolderID: params.ParentFolderID,
		IncludeDeleted: params.IncludeDeleted,
		AllFolders:     params.AllFolders,
	})
	if err != nil {
		return ListFoldersResult{}, fmt.Errorf("failed to list folders: %w", err)
	}

	folders := make([]Folder, len(resp.Folders))
	for i, folder := range resp.Folders {
		folders[i] = Folder{
			ID:             folder.ID,
			WorkspaceID:    folder.WorkspaceID,
			Name:           folder.Name,
			ParentFolderID: folder.ParentFolderID,
			Path:           folder.Path,
			CreatedAt:      folder.CreatedAt,
			CreatedBy:      folder.CreatedBy,
			UpdatedAt:      folder.UpdatedAt,
			IsDeleted:      folder.IsDeleted,
			Order:          folder.Order,
			FileCount:      folder.FileCount,
		}
	}

	return ListFoldersResult{
		Folders: folders,
	}, nil
}

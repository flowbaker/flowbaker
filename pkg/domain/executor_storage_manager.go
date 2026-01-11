package domain

import (
	"context"
	"errors"
	"io"
)

var (
	ErrUploadNotFound = errors.New("upload not found")
)

type ExecutorStorageManager interface {
	GetExecutionFile(ctx context.Context, params GetExecutionFileParams) (ExecutionWorkspaceFile, error)
	GetExecutionFileAsFileItem(ctx context.Context, params GetExecutionFileParams) (FileItem, error)
	PutExecutionFile(ctx context.Context, params PutExecutionFileParams) (FileItem, error)
	PersistExecutionFile(ctx context.Context, params PersistExecutionFileParams) error

	ListWorkspaceFiles(ctx context.Context, params ListWorkspaceFilesParams) (ListWorkspaceFilesResult, error)
	DeleteExecutionFile(ctx context.Context, params DeleteExecutionFileParams) error
	ListFolders(ctx context.Context, params ListFoldersParams) (ListFoldersResult, error)
}

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

type ListWorkspaceFilesParams struct {
	WorkspaceID string
	FolderIDs   []string
	ExcludeRoot bool
	Cursor      string
	Limit       int
}

type ListWorkspaceFilesResult struct {
	Files      []WorkspaceFile
	NextCursor string
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

package domain

import "time"

type FileItem struct {
	FileID      string `json:"file_id"`
	WorkspaceID string `json:"workspace_id"`
	ObjectKey   string `json:"key"`
	Name        string `json:"name"`
	SizeInBytes int64  `json:"size_in_bytes"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
}

type WorkspaceFile struct {
	UploadID    string
	UploadedBy  string
	WorkspaceID string
	FileName    string
	ContentType string
	Size        int64
	UploadedAt  time.Time
	ExpiresAt   *time.Time
	TagIDs      []string
	FolderID    *string
	FolderPath  string
}

type Folder struct {
	ID             string
	WorkspaceID    string
	Name           string
	ParentFolderID *string
	Path           string
	CreatedAt      time.Time
	CreatedBy      string
	UpdatedAt      time.Time
	IsDeleted      bool
	Order          int
	FileCount      int64 // Number of files in this folder
}

package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

type FileItem struct {
	FileID      string `json:"file_id"`
	WorkspaceID string `json:"workspace_id"`
	ObjectKey   string `json:"key"`
	Name        string `json:"name"`
	SizeInBytes int64  `json:"size_in_bytes"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
}

// UnmarshalJSON implements custom JSON unmarshaling for FileItem.
// This handles cases where the FileItem is received as a JSON string instead of an object,
// which can happen when expression evaluators stringify complex objects.
func (f *FileItem) UnmarshalJSON(data []byte) error {
	type fileItemAlias FileItem
	var item fileItemAlias
	if err := json.Unmarshal(data, &item); err == nil {
		*f = FileItem(item)
		return nil
	}

	var jsonString string
	if err := json.Unmarshal(data, &jsonString); err != nil {
		return fmt.Errorf("FileItem unmarshal failed: data is neither object nor string: %w", err)
	}

	var item2 fileItemAlias
	if err := json.Unmarshal([]byte(jsonString), &item2); err != nil {
		return fmt.Errorf("FileItem unmarshal failed: string is not valid JSON: %w", err)
	}

	*f = FileItem(item2)
	return nil
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

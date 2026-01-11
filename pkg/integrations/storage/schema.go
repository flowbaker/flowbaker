package storage

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	ActionDeleteFile  domain.IntegrationActionType = "delete_file"
	ActionGetFile     domain.IntegrationActionType = "get_file"
	ActionListFiles   domain.IntegrationActionType = "list_files"
	ActionPersistFile domain.IntegrationActionType = "persist_file"
)

var (
	Schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_FlowbakerStorage,
		Name:                 "Storage",
		Description:          "Manage files and folders in your workspace storage. Upload, download, list, and organize files seamlessly within your workflows.",
		CanTestConnection:    false, // Internal service - no external credentials needed
		CredentialProperties: []domain.NodeProperty{},
		Actions: []domain.IntegrationAction{
			{
				ID:          "list_files",
				ActionType:  ActionListFiles,
				Name:        "List Files",
				Description: "List files in your workspace with optional filtering",
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "Maximum number of files to return (1-1000, default: 1000)",
						Type:        domain.NodePropertyType_Integer,
						Required:    false,
						Placeholder: "1000",
					},
					{
						Key:                     "folder_id",
						Name:                    "Folder",
						Description:             "Folder to list files from (leave empty for root)",
						Type:                    domain.NodePropertyType_String,
						Required:                false,
						Peekable:                true,
						PeekableType:            StorageIntegrationPeekable_Folders,
						IsNonCredentialPeekable: true,
						ExpressionChoice:        true,
						PeekablePaginationType:  domain.PeekablePaginationType_Cursor,
					},
					{
						Key:                     "from_file_id",
						Name:                    "Start From File ID",
						Description:             "Continue listing from this file ID (for pagination)",
						Type:                    domain.NodePropertyType_String,
						Required:                false,
						Peekable:                true,
						PeekableType:            StorageIntegrationPeekable_Files,
						IsNonCredentialPeekable: true,
						ExpressionChoice:        true,
						PeekablePaginationType:  domain.PeekablePaginationType_Cursor,
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "folder_id",
								ValueKey:    "folder_id",
							},
						},
					},
				},
			},
			{
				ID:          "get_file",
				ActionType:  ActionGetFile,
				Name:        "Get File",
				Description: "Retrieve a file and its content for use in your workflow",
				Properties: []domain.NodeProperty{
					{
						Key:                     "file_id",
						Name:                    "File ID",
						Description:             "The unique identifier of the file to retrieve",
						Type:                    domain.NodePropertyType_String,
						Required:                true,
						Peekable:                true,
						PeekableType:            StorageIntegrationPeekable_Files,
						IsNonCredentialPeekable: true,
						ExpressionChoice:        true,
						PeekablePaginationType:  domain.PeekablePaginationType_Cursor,
					},
				},
			},
			{
				ID:          "delete_file",
				ActionType:  ActionDeleteFile,
				Name:        "Delete File",
				Description: "Permanently remove a file from your workspace storage",
				Properties: []domain.NodeProperty{
					{
						Key:                     "file_id",
						Name:                    "File ID",
						Description:             "The unique identifier of the file to delete",
						Type:                    domain.NodePropertyType_String,
						Required:                true,
						Peekable:                true,
						PeekableType:            StorageIntegrationPeekable_Files,
						IsNonCredentialPeekable: true,
						ExpressionChoice:        true,
						PeekablePaginationType:  domain.PeekablePaginationType_Cursor,
					},
				},
			},
			{
				ID:          "persist_file",
				ActionType:  ActionPersistFile,
				Name:        "Persist File",
				Description: "Remove expiration date from a file to prevent automatic deletion",
				Properties: []domain.NodeProperty{
					{
						Key:              "file",
						Name:             "File",
						Description:      "The file to persist (remove expiration date)",
						Type:             domain.NodePropertyType_File,
						Required:         true,
						ExpressionChoice: true,
					},
				},
			},
		},
	}
)

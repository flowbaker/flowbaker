package dropbox

import (
	"github.com/flowbaker/flowbaker/internal/domain"
)

var (
	DropboxSchema = domain.Integration{
		ID:          domain.IntegrationType_Dropbox,
		Name:        "Dropbox",
		Description: "Use Dropbox integration to upload and download files, create folders, and more.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:                "access_token",
				Name:               "Access Token",
				Description:        "The access token for the Dropbox API",
				Required:           true,
				Type:               domain.NodePropertyType_OAuth,
				OAuthType:          domain.OAuthTypeDropbox,
				IsApplicableToHTTP: true,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "download_file",
				Name:        "Download File",
				Description: "Download a file from Dropbox",
				ActionType:  IntegrationActionType_DownloadFile,
				Properties: []domain.NodeProperty{
					{
						Key:              "file_path",
						Name:             "File Path",
						Description:      "The path to the file to download",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DropboxIntegrationPeekable_Files,
						ExpressionChoice: true,
					},
				},
			},
			{
				ID:          "upload_file",
				Name:        "Upload File",
				Description: "Upload a file to Dropbox",
				ActionType:  IntegrationActionType_UploadFile,
				Properties: []domain.NodeProperty{
					{
						Key:              "file_path",
						Name:             "File Path",
						Description:      "The path to the file to upload",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DropboxIntegrationPeekable_Folders,
						ExpressionChoice: true,
					},

					{
						Key:         "file_content",
						Name:        "File Content",
						Description: "The content of the file to upload",
						Required:    true,
						Type:        domain.NodePropertyType_File,
					},
				},
			},
			{
				ID:          "move_file",
				Name:        "Move File",
				Description: "Move a file in Dropbox",
				ActionType:  IntegrationActionType_MoveFile,
				Properties: []domain.NodeProperty{
					{
						Key:              "from_path",
						Name:             "From Path",
						Description:      "The path to the file to move",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DropboxIntegrationPeekable_Files,
						ExpressionChoice: true,
					},
					{
						Key:              "to_path",
						Name:             "To Path",
						Description:      "The path to the file to move to",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DropboxIntegrationPeekable_Folders,
						ExpressionChoice: true,
					},
				},
			},
			{
				ID:          "copy_file",
				Name:        "Copy File",
				Description: "Copy a file in Dropbox",
				ActionType:  IntegrationActionType_CopyFile,
				Properties: []domain.NodeProperty{
					{
						Key:              "from_path",
						Name:             "From Path",
						Description:      "The path to the file to copy",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DropboxIntegrationPeekable_Files,
						ExpressionChoice: true,
					},
					{
						Key:          "to_path",
						Name:         "To Path",
						Description:  "The path to the file to copy to",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     false,
						PeekableType: DropboxIntegrationPeekable_Folders,
					},
				},
			},
			{
				ID:          "delete_file",
				Name:        "Delete File",
				Description: "Delete a file from Dropbox",
				ActionType:  IntegrationActionType_DeleteFile,
				Properties: []domain.NodeProperty{
					{
						Key:              "file_path",
						Name:             "File Path",
						Description:      "The path to the file to delete",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DropboxIntegrationPeekable_Files,
						ExpressionChoice: true,
					},
				},
			},
			{
				ID:          "create_folder",
				Name:        "Create Folder",
				Description: "Create a folder in Dropbox",
				ActionType:  IntegrationActionType_CreateFolder,
				Properties: []domain.NodeProperty{
					{
						Key:         "folder_path",
						Name:        "Folder Path",
						Description: "The path to the folder to create",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "move_folder",
				Name:        "Move Folder",
				Description: "Move a folder in Dropbox",
				ActionType:  IntegrationActionType_MoveFolder,
				Properties: []domain.NodeProperty{
					{
						Key:              "from_path",
						Name:             "From Path",
						Type:             domain.NodePropertyType_String,
						Description:      "The path to the folder to move",
						Required:         true,
						Peekable:         true,
						PeekableType:     DropboxIntegrationPeekable_Folders,
						ExpressionChoice: true,
					},
					{
						Key:         "to_path",
						Name:        "To Path",
						Type:        domain.NodePropertyType_String,
						Description: "The path to the folder to move to",
						Required:    true,
					},
				},
			},
			{
				ID:          "copy_folder",
				Name:        "Copy Folder",
				Description: "Copy a folder in Dropbox",
				ActionType:  IntegrationActionType_CopyFolder,
				Properties: []domain.NodeProperty{
					{
						Key:              "from_path",
						Name:             "From Path",
						Description:      "The path to the folder to copy",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DropboxIntegrationPeekable_Folders,
						ExpressionChoice: true,
					},
					{
						Key:         "to_path",
						Name:        "To Path",
						Description: "The path to the folder to copy to",
						Type:        domain.NodePropertyType_String,
						Required:    true,
					},
				},
			},
			{
				ID:          "delete_folder",
				Name:        "Delete Folder",
				Description: "Delete a folder from Dropbox",
				ActionType:  IntegrationActionType_DeleteFolder,
				Properties: []domain.NodeProperty{
					{
						Key:              "folder_path",
						Name:             "Folder Path",
						Description:      "The path to the folder to delete",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DropboxIntegrationPeekable_Folders,
						ExpressionChoice: true,
					},
				},
			},
		},
	}
)

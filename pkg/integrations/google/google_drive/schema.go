package googledrive

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	Schema = schema

	schema = domain.Integration{
		ID:                domain.IntegrationType_Drive,
		Name:              "Google Drive",
		Description:       "Use Google Drive API to manage files and folders",
		CanTestConnection: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:               "token",
				Name:              "Token",
				Description:       "Google Account Authentication Token",
				Required:          false,
				Type:              domain.NodePropertyType_OAuth,
				OAuthType:         domain.OAuthTypeGoogle,
				IsCustomOAuthable: true,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "upload_file",
				Name:        "Upload File",
				ActionType:  IntegrationActionType_UploadFile,
				Description: "Upload a file to Google Drive root folder",
				Properties: []domain.NodeProperty{
					{
						Key:         "file_name",
						Name:        "File Name",
						Description: "The desired name for the uploaded file. If not provided, the file name will be used.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "file",
						Name:        "File",
						Description: "The file to upload",
						Required:    true,
						Type:        domain.NodePropertyType_File,
					},
				},
			},
			{
				ID:          "upload_file_to_folder",
				Name:        "Upload File to Folder",
				ActionType:  IntegrationActionType_UploadFileToFolder,
				Description: "Upload a file to a specific folder in Google Drive",
				Properties: []domain.NodeProperty{
					{
						Key:         "file_name",
						Name:        "File Name",
						Description: "The desired name for the uploaded file",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "file",
						Name:        "File",
						Description: "The file to upload",
						Required:    true,
						Type:        domain.NodePropertyType_File,
					},
					{
						Key:                    "folder_id",
						Name:                   "Folder",
						Description:            "The ID of the folder to upload the file into",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Folders,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

						ExpressionChoice: true,
					},
				},
			},
			{
				ID:          "download_file",
				Name:        "Download File",
				ActionType:  IntegrationActionType_DownloadFile,
				Description: "Find a file by its exact name in Google Drive (returns the first match)",
				Properties: []domain.NodeProperty{
					{
						Key:                    "file_id",
						Name:                   "File",
						Description:            "The file to download",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Files,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

						ExpressionChoice: true,
					},
				},
			},
			{
				ID:          "copy_file",
				Name:        "Copy File",
				ActionType:  IntegrationActionType_CopyFile,
				Description: "Copies a file to a new name and optionally to a new folder. Requires both the source file name/ID and the new file name.",
				Properties: []domain.NodeProperty{
					{
						Key:                    "file_id",
						Name:                   "Source File ID",
						Description:            "The name or ID of the file to copy. You can provide the file name and it will be resolved automatically.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Files,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
					{
						Key:         "new_file_name",
						Name:        "New File Name",
						Description: "The name for the copied file.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:                    "destination_folder_id",
						Name:                   "Destination Folder ID (Optional)",
						Description:            "Destination folder, My Drive root, or Shared Drive.",
						Required:               false,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Folders,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
				},
			},
			{
				ID:          "create_file_from_text",
				Name:        "Create File From Text",
				ActionType:  IntegrationActionType_CreateFileFromText,
				Description: "Creates a new file with the given text content.",
				Properties: []domain.NodeProperty{
					{
						Key:         "file_name",
						Name:        "File Name",
						Description: "The name for the new file.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "content",
						Name:        "Content",
						Description: "The text content for the file.",
						Required:    true,
						Type:        domain.NodePropertyType_String, // Consider _LongString or similar if available
					},
					{
						Key:         "mime_type",
						Name:        "MIME Type (Optional)",
						Description: "The MIME type of the file (e.g., 'text/plain', 'text/html'). Defaults to 'text/plain'.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:                    "folder_id",
						Name:                   "Parent Folder ID (Optional)",
						Description:            "The ID of the folder to create the file in. If empty, creates in the root.",
						Required:               false,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Folders,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
				},
			},
			{
				ID:          "delete_file",
				Name:        "Delete File",
				ActionType:  IntegrationActionType_DeleteFile,
				Description: "Permanently deletes a file.",
				Properties: []domain.NodeProperty{
					{
						Key:                    "file_id",
						Name:                   "File ID",
						Description:            "The ID of the file to delete.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Files,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
				},
			},
			{
				ID:          "move_file",
				Name:        "Move File",
				ActionType:  IntegrationActionType_MoveFile,
				Description: "Moves a file to a different folder.",
				Properties: []domain.NodeProperty{
					{
						Key:                    "file_id",
						Name:                   "File ID",
						Description:            "The ID of the file to move.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Files,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
					{
						Key:                    "destination_folder_id",
						Name:                   "Destination Folder ID",
						Description:            "Destination folder, My Drive root, or Shared Drive.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Folders,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
				},
			},
			{
				ID:          "share_file",
				Name:        "Share File",
				ActionType:  IntegrationActionType_ShareFile,
				Description: "Shares a file with specified permissions.",
				Properties: []domain.NodeProperty{
					{
						Key:                    "file_id",
						Name:                   "File ID",
						Description:            "The ID of the file to share.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Files,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
					{
						Key:         "role",
						Name:        "Role",
						Description: "The role to grant (e.g., 'reader', 'writer', 'commenter').",
						Required:    true,
						Type:        domain.NodePropertyType_String, // Or String with validation
						Options: []domain.NodePropertyOption{ // Example options
							{Label: "Reader", Value: "reader"},
							{Label: "Commenter", Value: "commenter"},
							{Label: "Writer", Value: "writer"},
						},
					},
					{
						Key:         "type",
						Name:        "Type",
						Description: "The type of grantee (e.g., 'user', 'group', 'domain', 'anyone').",
						Required:    true,
						Type:        domain.NodePropertyType_String, // Or String with validation
						Options: []domain.NodePropertyOption{
							{Label: "User", Value: "user"},
							{Label: "Group", Value: "group"},
							{Label: "Domain", Value: "domain"},
							{Label: "Anyone", Value: "anyone"},
						},
					},
					{
						Key:         "email_address",
						Name:        "Email Address (Optional)",
						Description: "Email address for 'user' or 'group' type. Not used for 'anyone' or 'domain'.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "domain",
						Name:        "Domain (Optional)",
						Description: "The domain name for 'domain' type.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "allow_file_discovery",
						Name:        "Allow File Discovery (for 'anyone')",
						Description: "Whether the file can be discovered by search engines when shared with 'anyone'.",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
				},
			},
			{
				ID:          "update_file",
				Name:        "Update File",
				ActionType:  IntegrationActionType_UpdateFile,
				Description: "Updates a file's metadata or content.",
				Properties: []domain.NodeProperty{
					{
						Key:                    "file_id",
						Name:                   "File ID",
						Description:            "The ID of the file to update.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Files,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
					{
						Key:         "new_file_name",
						Name:        "New File Name (Optional)",
						Description: "The new name for the file.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					// { // TODO: Add FileContent as domain.NodePropertyType_File once available and linter issues with domain.FileItem are resolved
					// 	Key:         "file_content",
					// 	Name:        "File Content (Optional)",
					// 	Description: "New content for the file. If provided, updates the file's content.",
					// 	Required:    false,
					// 	Type:        domain.NodePropertyType_File, // Assuming a File type for input
					// },
				},
			},
			{
				ID:          "search_files_and_folders",
				Name:        "Search Files and Folders",
				ActionType:  IntegrationActionType_SearchFilesAndFolders,
				Description: "Searches for files and folders by name containment.",
				Properties: []domain.NodeProperty{
					{
						Key:         "name_contains",
						Name:        "Name",
						Description: "Text to search for within file or folder names.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "page_size",
						Name:        "Page Size",
						Description: "Number of results per page.",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "page_token",
						Name:        "Page Token (Optional)",
						Description: "Token for the next page of results, from a previous search.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "create_folder",
				Name:        "Create Folder",
				ActionType:  IntegrationActionType_CreateFolder,
				Description: "Creates a new folder.",
				Properties: []domain.NodeProperty{
					{
						Key:         "folder_name",
						Name:        "Folder Name",
						Description: "The name for the new folder.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:                    "parent_folder_id",
						Name:                   "Parent Folder ID (Optional)",
						Description:            "The ID of the parent folder. If empty, creates in the root.",
						Required:               false,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Folders,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
				},
			},
			{
				ID:          "delete_folder",
				Name:        "Delete Folder",
				ActionType:  IntegrationActionType_DeleteFolder,
				Description: "Permanently deletes a folder.",
				Properties: []domain.NodeProperty{
					{
						Key:                    "folder_id",
						Name:                   "Folder ID",
						Description:            "The ID of the folder to delete.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_Folders,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
				},
			},
			{
				ID:          "share_folder",
				Name:        "Share Folder",
				ActionType:  IntegrationActionType_ShareFolder,
				Description: "Shares a folder with specified permissions.",
				Properties: []domain.NodeProperty{
					{
						Key:                    "folder_id",
						Name:                   "Folder ID",
						Description:            "The folder or Shared Drive to share (My Drive root cannot be shared).",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_ShareableFolders,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
					{
						Key:         "role",
						Name:        "Role",
						Description: "The role to grant (e.g., 'reader', 'writer', 'organizer').",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Reader", Value: "reader"},
							{Label: "Commenter", Value: "commenter"},
							{Label: "Writer", Value: "writer"},
							{Label: "File Organizer", Value: "fileOrganizer"}, // Role specific to organizing files within folder
							{Label: "Organizer", Value: "organizer"},          // Broader organizer role
						},
					},
					{
						Key:         "type",
						Name:        "Type",
						Description: "The type of grantee (e.g., 'user', 'group', 'domain', 'anyone').",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "User", Value: "user"},
							{Label: "Group", Value: "group"},
							{Label: "Domain", Value: "domain"},
							{Label: "Anyone", Value: "anyone"},
						},
					},
					{
						Key:         "email_address",
						Name:        "Email Address (Optional)",
						Description: "Email address for 'user' or 'group' type.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "domain",
						Name:        "Domain (Optional)",
						Description: "The domain name for 'domain' type.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "allow_file_discovery",
						Name:        "Allow File Discovery (for 'anyone')",
						Description: "Whether files in this folder can be discovered by search engines when shared with 'anyone'.",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
				},
			},
			{
				ID:                            "create_shared_drive",
				Name:                          "Create Shared Drive",
				ActionType:                    IntegrationActionType_CreateSharedDrive,
				Description:                   "Creates a new Shared Drive.",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "request_id",
						Name:        "Request ID",
						Description: "A unique ID for the request (e.g., a UUID). Ensures idempotency.",
						Required:    true,
						Type:        domain.NodePropertyType_String, // User should generate this
					},
					{
						Key:         "name",
						Name:        "Shared Drive Name",
						Description: "The name for the new Shared Drive.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					// TODO: Add other optional properties like ThemeID, ColorRgb from drive.Drive object if needed
				},
			},
			{
				ID:                            "delete_shared_drive",
				Name:                          "Delete Shared Drive",
				ActionType:                    IntegrationActionType_DeleteSharedDrive,
				Description:                   "Deletes a Shared Drive. The Shared Drive must usually be empty.",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:                    "shared_drive_id",
						Name:                   "Shared Drive ID",
						Description:            "The ID of the Shared Drive to delete.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_SharedDrives,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
				},
			},
			{
				ID:                            "get_shared_drive",
				Name:                          "Get Shared Drive",
				ActionType:                    IntegrationActionType_GetSharedDrive,
				Description:                   "Retrieves information about a specific Shared Drive.",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:                    "shared_drive_id",
						Name:                   "Shared Drive ID",
						Description:            "The ID of the Shared Drive to retrieve.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_SharedDrives,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
				},
			},
			{
				ID:                            "list_shared_drives",
				Name:                          "List Shared Drives",
				ActionType:                    IntegrationActionType_ListSharedDrives,
				Description:                   "Lists Shared Drives accessible to the user.",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "page_size",
						Name:        "Page Size (Optional)",
						Description: "Maximum number of Shared Drives to return.",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "page_token",
						Name:        "Page Token (Optional)",
						Description: "Token for the next page of results from a previous list operation.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					// { // Example: Query parameter for Drives.List if supported and needed
					// 	Key:         "q",
					// 	Name:        "Query (Optional)",
					// 	Description: "Query string for filtering Shared Drives (e.g., "name contains 'Team'").",
					// 	Required:    false,
					// 	Type:        domain.NodePropertyType_String,
					// },
				},
			},
			{
				ID:                            "update_shared_drive",
				Name:                          "Update Shared Drive",
				ActionType:                    IntegrationActionType_UpdateSharedDrive,
				Description:                   "Updates an existing Shared Drive's metadata (e.g., name, theme).",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:                    "shared_drive_id",
						Name:                   "Shared Drive ID",
						Description:            "The ID of the Shared Drive to update.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           GoogleDrivePeekable_SharedDrives,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

					},
					{
						Key:         "name",
						Name:        "New Name (Optional)",
						Description: "The new name for the Shared Drive.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "theme_id",
						Name:        "New Theme ID (Optional)",
						Description: "The ID of the new theme for the Shared Drive.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					// TODO: Add other updatable fields like ColorRgb if necessary
				},
			},
		},
		/*Triggers: []domain.IntegrationTrigger{
			{
				Name:        "File Changed",
				Description: "Triggers when a specific file's content or metadata is changed.",
				EventType:   GoogleDriveTriggerType_FileChanged,
				Properties: []domain.NodeProperty{
					{
						Key:          "file_id",
						Name:         "File",
						Description:  "The specific file to monitor for changes.",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GoogleDrivePeekable_Files,
					},
				},
			},
			{
				Name:        "Folder Changed",
				Description: "Triggers when a file is added, modified, or removed within a specific folder.",
				EventType:   GoogleDriveTriggerType_FolderChanged,
				Properties: []domain.NodeProperty{
					{
						Key:          "folder_id",
						Name:         "Folder",
						Description:  "The specific folder to monitor for changes.",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GoogleDrivePeekable_Folders,
					},
				},
			},
		},*/
	}
)

package googledrive

import (
	"context"
	"fmt"
	"strings"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const (
	IntegrationActionType_UploadFile            domain.IntegrationActionType = "upload_file"
	IntegrationActionType_UploadFileToFolder    domain.IntegrationActionType = "upload_file_to_folder"
	IntegrationActionType_DownloadFile          domain.IntegrationActionType = "download_file"
	IntegrationActionType_CopyFile              domain.IntegrationActionType = "copy_file"
	IntegrationActionType_CreateFileFromText    domain.IntegrationActionType = "create_file_from_text"
	IntegrationActionType_DeleteFile            domain.IntegrationActionType = "delete_file"
	IntegrationActionType_MoveFile              domain.IntegrationActionType = "move_file"
	IntegrationActionType_ShareFile             domain.IntegrationActionType = "share_file"
	IntegrationActionType_UpdateFile            domain.IntegrationActionType = "update_file"
	IntegrationActionType_SearchFilesAndFolders domain.IntegrationActionType = "search_files_and_folders"
	IntegrationActionType_CreateFolder          domain.IntegrationActionType = "create_folder"
	IntegrationActionType_DeleteFolder          domain.IntegrationActionType = "delete_folder"
	IntegrationActionType_ShareFolder           domain.IntegrationActionType = "share_folder"
	IntegrationActionType_CreateSharedDrive     domain.IntegrationActionType = "create_shared_drive"
	IntegrationActionType_DeleteSharedDrive     domain.IntegrationActionType = "delete_shared_drive"
	IntegrationActionType_GetSharedDrive        domain.IntegrationActionType = "get_shared_drive"
	IntegrationActionType_ListSharedDrives      domain.IntegrationActionType = "list_shared_drives"
	IntegrationActionType_UpdateSharedDrive     domain.IntegrationActionType = "update_shared_drive"
)

const (
	GoogleDriveTriggerType_FileChanged   domain.IntegrationTriggerEventType = "google_drive_file_changed"
	GoogleDriveTriggerType_FolderChanged domain.IntegrationTriggerEventType = "google_drive_folder_changed"
)

const (
	GoogleDrivePeekable_Folders          domain.IntegrationPeekableType = "folders"
	GoogleDrivePeekable_Files            domain.IntegrationPeekableType = "files"
	GoogleDrivePeekable_SharedDrives     domain.IntegrationPeekableType = "shared_drives"
	GoogleDrivePeekable_ShareableFolders domain.IntegrationPeekableType = "shareable_folders"
)

type GoogleDriveIntegrationCreator struct {
	credentialGetter       domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
}

type GoogleDriveIntegrationCreatorDeps struct {
	CredentialGetter       domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	ParameterBinder        domain.IntegrationParameterBinder
	ExecutorStorageManager domain.ExecutorStorageManager
}

func NewGoogleDriveIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &GoogleDriveIntegrationCreator{
		credentialGetter:       managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}
}

func (c *GoogleDriveIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewGoogleDriveIntegration(ctx, GoogleDriveIntegrationDependencies{
		ParameterBinder:        c.binder,
		CredentialID:           p.CredentialID,
		CredentialGetter:       c.credentialGetter,
		ExecutorStorageManager: c.executorStorageManager,
		WorkspaceID:            p.WorkspaceID,
	})
}

type GoogleDriveIntegrationDependencies struct {
	ParameterBinder        domain.IntegrationParameterBinder
	CredentialID           string
	CredentialGetter       domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	ExecutorStorageManager domain.ExecutorStorageManager
	WorkspaceID            string
}

type GoogleDriveIntegration struct {
	credentialGetter       domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder                 domain.IntegrationParameterBinder
	driveService           *drive.Service
	executorStorageManager domain.ExecutorStorageManager
	workspaceID            string
	actionManager          *domain.IntegrationActionManager
	peekFuncs              map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

func NewGoogleDriveIntegration(ctx context.Context, deps GoogleDriveIntegrationDependencies) (*GoogleDriveIntegration, error) {
	integration := &GoogleDriveIntegration{
		credentialGetter:       deps.CredentialGetter,
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
		workspaceID:            deps.WorkspaceID,
	}

	oauthAccount, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get decrypted OAuth credential: %w", err)
	}
	token := &oauth2.Token{
		AccessToken:  oauthAccount.AccessToken,
		RefreshToken: oauthAccount.RefreshToken,
		Expiry:       oauthAccount.Expiry,
		TokenType:    "Bearer",
	}

	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(token))

	if integration.driveService == nil {
		integration.driveService, err = drive.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			return nil, fmt.Errorf("failed to create drive service: %w", err)
		}
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_UploadFile, integration.UploadFile).
		AddPerItem(IntegrationActionType_UploadFileToFolder, integration.UploadFileToFolder).
		AddPerItem(IntegrationActionType_DownloadFile, integration.DownloadFile).
		AddPerItem(IntegrationActionType_CopyFile, integration.CopyFile).
		AddPerItem(IntegrationActionType_CreateFileFromText, integration.CreateFileFromText).
		AddPerItem(IntegrationActionType_DeleteFile, integration.DeleteFile).
		AddPerItem(IntegrationActionType_MoveFile, integration.MoveFile).
		AddPerItem(IntegrationActionType_ShareFile, integration.ShareFile).
		AddPerItem(IntegrationActionType_UpdateFile, integration.UpdateFile).
		AddPerItem(IntegrationActionType_SearchFilesAndFolders, integration.SearchFilesAndFolders).
		AddPerItem(IntegrationActionType_CreateFolder, integration.CreateFolder).
		AddPerItem(IntegrationActionType_DeleteFolder, integration.DeleteFolder).
		AddPerItem(IntegrationActionType_ShareFolder, integration.ShareFolder).
		AddPerItem(IntegrationActionType_CreateSharedDrive, integration.CreateSharedDrive).
		AddPerItem(IntegrationActionType_DeleteSharedDrive, integration.DeleteSharedDrive).
		AddPerItem(IntegrationActionType_GetSharedDrive, integration.GetSharedDrive).
		AddPerItem(IntegrationActionType_ListSharedDrives, integration.ListSharedDrives).
		AddPerItem(IntegrationActionType_UpdateSharedDrive, integration.UpdateSharedDrive)

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		GoogleDrivePeekable_Folders:          integration.PeekFoldersWithRoot,
		GoogleDrivePeekable_ShareableFolders: integration.PeekFolders,
		GoogleDrivePeekable_Files:            integration.PeekFiles,
		GoogleDrivePeekable_SharedDrives:     integration.PeekSharedDrives,
	}

	integration.actionManager = actionManager
	integration.peekFuncs = peekFuncs

	return integration, nil
}

func (g *GoogleDriveIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return g.actionManager.Run(ctx, params.ActionType, params)
}

func (g *GoogleDriveIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := g.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found for type '%s'", params.PeekableType)
	}
	return peekFunc(ctx, params)
}

func (g *GoogleDriveIntegration) PeekFolders(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	var results []domain.PeekResultItem
	limit := p.GetLimitWithMax(20, 100)

	myDriveQuery := "mimeType='application/vnd.google-apps.folder' and trashed=false"
	myDriveCall := g.driveService.Files.List().
		Q(myDriveQuery).
		Fields("nextPageToken, files(id, name, parents, owners)").
		OrderBy("name").
		PageSize(int64(limit)).
		IncludeItemsFromAllDrives(true).
		SupportsAllDrives(true)

	if p.Pagination.PageToken != "" {
		myDriveCall = myDriveCall.PageToken(p.Pagination.PageToken)
	}

	myDriveFolders, err := myDriveCall.Context(ctx).Do()
	nextPageToken := ""
	hasMore := false

	if err == nil {
		nextPageToken = myDriveFolders.NextPageToken
		hasMore = nextPageToken != ""

		for _, folder := range myDriveFolders.Files {
			isMyFolder := false
			for _, owner := range folder.Owners {
				if owner.Me {
					isMyFolder = true
					break
				}
			}

			if isMyFolder {
				results = append(results, domain.PeekResultItem{
					Key:     folder.Name,
					Value:   folder.Id,
					Content: folder.Name,
				})
			}
		}
	}

	if p.Pagination.PageToken == "" {
		sharedDrives, err := g.driveService.Drives.List().
			PageSize(int64(limit)).
			Fields("nextPageToken, drives(id, name)").
			Context(ctx).
			Do()

		if err == nil {

			for _, drive := range sharedDrives.Drives {
				results = append(results, domain.PeekResultItem{
					Key:     drive.Name,
					Value:   drive.Id,
					Content: fmt.Sprintf("%s (Shared Drive)", drive.Name),
				})
			}
			if sharedDrives.NextPageToken != "" {
				hasMore = true
			}
		} else {
			log.Error().Err(err).Msg("[GOOGLE DRIVE PEEK] Shared Drives API error")
		}

		sharedQuery := "mimeType='application/vnd.google-apps.folder' and sharedWithMe and trashed=false"
		sharedFolders, err := g.driveService.Files.List().
			Q(sharedQuery).
			Fields("nextPageToken, files(id, name, owners)").
			PageSize(int64(limit)).
			Context(ctx).
			Do()

		if err == nil {

			for _, folder := range sharedFolders.Files {
				ownerName := "Unknown"
				if len(folder.Owners) > 0 {
					ownerName = folder.Owners[0].DisplayName
				}
				results = append(results, domain.PeekResultItem{
					Key:     folder.Name,
					Value:   folder.Id,
					Content: fmt.Sprintf("%s (Shared by %s)", folder.Name, ownerName),
				})
			}
			if sharedFolders.NextPageToken != "" {
				hasMore = true
			}
		} else {
			log.Error().Err(err).Msg("[GOOGLE DRIVE PEEK] Shared with me API error")
		}
	}

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextPageToken: nextPageToken,
			HasMore:       hasMore,
		},
	}

	result.Pagination.NextPageToken = nextPageToken
	result.Pagination.HasMore = hasMore

	return result, nil
}

func (g *GoogleDriveIntegration) PeekFoldersWithRoot(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	shareableResult, err := g.PeekFolders(ctx, p)
	if err != nil {
		return domain.PeekResult{}, err
	}

	results := shareableResult.Result

	if p.Pagination.PageToken == "" {
		results = append([]domain.PeekResultItem{
			{
				Key:     "My Drive",
				Value:   "root",
				Content: "My Drive (Root)",
			},
		}, results...)
	}

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextPageToken: shareableResult.Pagination.NextPageToken,
			HasMore:       shareableResult.Pagination.HasMore,
		},
	}

	result.Pagination.NextPageToken = shareableResult.Pagination.NextPageToken
	result.Pagination.HasMore = shareableResult.Pagination.HasMore

	return result, nil
}

// fetchSharedDriveFolders retrieves all folders from all shared drives
func (g *GoogleDriveIntegration) fetchSharedDriveFolders(ctx context.Context) ([]domain.PeekResultItem, error) {
	var results []domain.PeekResultItem

	sharedDrives, err := g.driveService.Drives.List().
		PageSize(100).
		Fields("drives(id, name)").
		Context(ctx).
		Do()

	if err != nil || sharedDrives == nil {
		return results, err
	}

	for _, drive := range sharedDrives.Drives {
		driveFolders, err := g.fetchFoldersFromDrive(ctx, drive.Id, drive.Name)
		if err == nil {
			results = append(results, driveFolders...)
		}
	}

	return results, nil
}

// fetchFoldersFromDrive retrieves all folders from a specific drive
func (g *GoogleDriveIntegration) fetchFoldersFromDrive(ctx context.Context, driveId, driveName string) ([]domain.PeekResultItem, error) {
	var results []domain.PeekResultItem

	query := "mimeType='application/vnd.google-apps.folder' and trashed=false"
	pageToken := ""

	for {
		folders, nextToken, err := g.fetchFolderPage(ctx, driveId, query, pageToken)
		if err != nil {
			return results, err
		}

		for _, folder := range folders {
			displayName := g.formatFolderPath(folder, driveId, driveName)
			results = append(results, domain.PeekResultItem{
				Key:     folder.Name,
				Value:   folder.Id,
				Content: displayName,
			})
		}

		if nextToken == "" {
			break
		}
		pageToken = nextToken
	}

	return results, nil
}

// fetchFolderPage retrieves a single page of folders
func (g *GoogleDriveIntegration) fetchFolderPage(ctx context.Context, driveId, query, pageToken string) ([]*drive.File, string, error) {
	listCall := g.driveService.Files.List().
		Q(query).
		Fields("nextPageToken, files(id, name, parents)").
		OrderBy("name").
		PageSize(100).
		IncludeItemsFromAllDrives(true).
		SupportsAllDrives(true).
		Corpora("drive").
		DriveId(driveId)

	if pageToken != "" {
		listCall = listCall.PageToken(pageToken)
	}

	result, err := listCall.Context(ctx).Do()
	if err != nil {
		return nil, "", err
	}

	return result.Files, result.NextPageToken, nil
}

// formatFolderPath creates a display path for a folder
func (g *GoogleDriveIntegration) formatFolderPath(folder *drive.File, driveId, driveName string) string {
	if len(folder.Parents) > 0 && folder.Parents[0] == driveId {
		return fmt.Sprintf("%s / %s", driveName, folder.Name)
	}
	return fmt.Sprintf("%s / ... / %s", driveName, folder.Name)
}

func (g *GoogleDriveIntegration) PeekFiles(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	var results []domain.PeekResultItem

	incomingPageToken := p.Pagination.PageToken

	limit := p.GetLimitWithMax(20, 100)

	query := "trashed=false and mimeType!='application/vnd.google-apps.folder'"
	listCall := g.driveService.Files.List().
		Q(query).
		Fields("nextPageToken, files(id, name, mimeType, owners, driveId)").
		OrderBy("modifiedTime desc").
		PageSize(int64(limit)).
		IncludeItemsFromAllDrives(true).
		SupportsAllDrives(true)

	if incomingPageToken != "" {
		listCall = listCall.PageToken(incomingPageToken)
	}

	filesList, err := listCall.Context(ctx).Do()
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list files: %w", err)
	}

	driveNameMap := make(map[string]string)
	if sharedDrives, driveErr := g.driveService.Drives.List().PageSize(100).Context(ctx).Do(); driveErr == nil {
		for _, drive := range sharedDrives.Drives {
			driveNameMap[drive.Id] = drive.Name
		}
	}

	for _, file := range filesList.Files {
		location := "Unknown"

		if file.DriveId != "" {
			if driveName, exists := driveNameMap[file.DriveId]; exists {
				location = driveName
			} else {
				location = fmt.Sprintf("Shared Drive (%s)", file.DriveId)
			}
		} else {
			for _, owner := range file.Owners {
				if owner.Me {
					location = "My Drive"
					break
				} else {
					location = fmt.Sprintf("Shared by %s", owner.DisplayName)
				}
			}
		}

		results = append(results, domain.PeekResultItem{
			Key:     file.Name,
			Value:   file.Id,
			Content: fmt.Sprintf("%s (%s)", file.Name, location),
		})
	}

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextPageToken: filesList.NextPageToken,
			HasMore:       filesList.NextPageToken != "",
		},
	}

	result.Pagination.NextPageToken = filesList.NextPageToken
	result.Pagination.HasMore = filesList.NextPageToken != ""

	return result, nil
}

func (g *GoogleDriveIntegration) PeekSharedDrives(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 100)

	listCall := g.driveService.Drives.List().
		PageSize(int64(limit)).
		Fields("nextPageToken, drives(id, name, capabilities)")

	if p.Pagination.PageToken != "" {
		listCall = listCall.PageToken(p.Pagination.PageToken)
	}

	driveList, err := listCall.Context(ctx).Do()
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list shared drives: %w", err)
	}

	var results []domain.PeekResultItem
	for _, driveItem := range driveList.Drives {
		accessLevel := "Viewer"
		if driveItem.Capabilities != nil {
			if driveItem.Capabilities.CanManageMembers {
				accessLevel = "Manager"
			} else if driveItem.Capabilities.CanAddChildren {
				accessLevel = "Contributor"
			} else if driveItem.Capabilities.CanComment {
				accessLevel = "Commenter"
			}
		}

		results = append(results, domain.PeekResultItem{
			Key:     driveItem.Name,
			Value:   driveItem.Id,
			Content: fmt.Sprintf("%s (%s access)", driveItem.Name, accessLevel),
		})
	}

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextPageToken: driveList.NextPageToken,
			HasMore:       driveList.NextPageToken != "",
		},
	}

	result.Pagination.NextPageToken = driveList.NextPageToken
	result.Pagination.HasMore = driveList.NextPageToken != ""

	return result, nil
}

type UploadFileParams struct {
	FileName string          `json:"file_name"`
	MimeType string          `json:"mime_type,omitempty"`
	File     domain.FileItem `json:"file"`
}

func (g *GoogleDriveIntegration) UploadFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p UploadFileParams

	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for upload file: %w", err)
	}

	if p.FileName == "" {
		p.FileName = p.File.Name
	}

	fileMetadata := &drive.File{
		Name:     p.FileName,
		MimeType: p.MimeType,
	}

	executionFile, err := g.executorStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
		WorkspaceID: g.workspaceID,
		UploadID:    p.File.FileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get execution file: %w", err)
	}

	uploadedFile, err := g.driveService.Files.Create(fileMetadata).
		Media(executionFile.Reader).
		Fields("id, name, webViewLink").
		SupportsAllDrives(true).
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to upload file to Google Drive: %w", err)
	}

	return uploadedFile, nil
}

type UploadFileToFolderParams struct {
	FileName string          `json:"file_name"`
	FolderID string          `json:"folder_id"`
	MimeType string          `json:"mime_type,omitempty"`
	File     domain.FileItem `json:"file"`
}

func (g *GoogleDriveIntegration) UploadFileToFolder(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p UploadFileToFolderParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for upload file to folder: %w", err)
	}

	if p.FileName == "" || p.FolderID == "" {
		return nil, fmt.Errorf("file name or folder ID is empty")
	}

	executionFile, err := g.executorStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
		WorkspaceID: g.workspaceID,
		UploadID:    p.File.FileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file from object storage: %w", err)
	}

	fileMetadata := &drive.File{
		Name:     p.FileName,
		MimeType: p.MimeType,
		Parents:  []string{p.FolderID},
	}

	uploadedFile, err := g.driveService.Files.Create(fileMetadata).
		Media(executionFile.Reader).
		Fields("id, name, webViewLink, parents").
		SupportsAllDrives(true).
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to upload file to Google Drive: %w", err)
	}

	return uploadedFile, nil
}

type DownloadFileParams struct {
	FileID string `json:"file_id"`
}

func (g *GoogleDriveIntegration) DownloadFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p DownloadFileParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for download file: %w", err)
	}
	if p.FileID == "" {
		return nil, fmt.Errorf("file id is empty")
	}

	file, err := g.driveService.Files.Get(p.FileID).
		Fields("id, name, mimeType, size").
		SupportsAllDrives(true).
		SupportsTeamDrives(true).
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	resp, err := g.driveService.Files.Get(p.FileID).
		SupportsAllDrives(true).
		SupportsTeamDrives(true).
		Context(ctx).Download()
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	fileItem, err := g.executorStorageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
		WorkspaceID:  g.workspaceID,
		UploadedBy:   g.workspaceID,
		OriginalName: file.Name,
		SizeInBytes:  file.Size,
		ContentType:  file.MimeType,
		Reader:       resp.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store file in object storage: %w", err)
	}

	googleDriveFileResult := GoogleDriveFileResult{
		File: fileItem,
	}

	return googleDriveFileResult, nil
}

type GoogleDriveFileResult struct {
	File domain.FileItem `json:"file"`
}

type CopyFileParams struct {
	FileID              string `json:"file_id"`
	NewFileName         string `json:"new_file_name"`
	DestinationFolderID string `json:"destination_folder_id,omitempty"`
}

func (g *GoogleDriveIntegration) CopyFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p CopyFileParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for copy file: %w", err)
	}
	if p.FileID == "" || p.NewFileName == "" {
		return nil, fmt.Errorf("file ID or new file name is empty")
	}

	copiedFile := &drive.File{Name: p.NewFileName}

	if p.DestinationFolderID != "" {
		driveInfo, driveErr := g.driveService.Drives.Get(p.DestinationFolderID).
			Fields("id").
			Context(ctx).
			Do()

		if driveErr == nil {
			copiedFile.Parents = []string{driveInfo.Id}
		} else {
			copiedFile.Parents = []string{p.DestinationFolderID}
		}
	}

	driveFile, err := g.driveService.Files.Copy(p.FileID, copiedFile).
		SupportsAllDrives(true).
		Fields("id, name, webViewLink, parents, driveId").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}
	return driveFile, nil
}

type CreateFileFromTextParams struct {
	FileName string `json:"file_name"`
	Content  string `json:"content"`
	MimeType string `json:"mime_type,omitempty"`
	FolderID string `json:"folder_id,omitempty"`
}

func (g *GoogleDriveIntegration) CreateFileFromText(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p CreateFileFromTextParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for create file from text: %w", err)
	}
	if p.FileName == "" {
		return nil, fmt.Errorf("file name is empty")
	}

	mimeType := "text/plain"
	if p.MimeType != "" {
		mimeType = p.MimeType
	}
	fileMetadata := &drive.File{
		Name:     p.FileName,
		MimeType: mimeType,
	}
	if p.FolderID != "" {
		fileMetadata.Parents = []string{p.FolderID}
	}
	createdFile, err := g.driveService.Files.Create(fileMetadata).
		Media(strings.NewReader(p.Content)).
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create file from text: %w", err)
	}
	return createdFile, nil
}

type DeleteFileParams struct {
	FileID string `json:"file_id"`
}

func (g *GoogleDriveIntegration) DeleteFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p DeleteFileParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for delete file: %w", err)
	}
	if p.FileID == "" {
		return nil, fmt.Errorf("file ID is empty")
	}

	fileInfo, err := g.driveService.Files.Get(p.FileID).
		Fields("id, name, mimeType, trashed").
		SupportsAllDrives(true).
		SupportsTeamDrives(true).
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info before deletion: %w", err)
	}

	err = g.driveService.Files.Delete(p.FileID).
		SupportsAllDrives(true).
		SupportsTeamDrives(true).
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}
	return fileInfo, nil
}

type MoveFileParams struct {
	FileID              string `json:"file_id"`
	DestinationFolderID string `json:"destination_folder_id"`
}

func (g *GoogleDriveIntegration) MoveFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p MoveFileParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for move file: %w", err)
	}
	if p.FileID == "" || p.DestinationFolderID == "" {
		return nil, fmt.Errorf("file ID or destination folder ID is empty")
	}

	file, err := g.driveService.Files.Get(p.FileID).
		Fields("id, name, mimeType, parents, driveId").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	isFolder := file.MimeType == "application/vnd.google-apps.folder"

	var destIsSharedDrive bool
	var destDriveId string
	var destDriveName string

	driveInfo, driveErr := g.driveService.Drives.Get(p.DestinationFolderID).
		Fields("id, name").
		Context(ctx).
		Do()

	if driveErr == nil {
		destIsSharedDrive = true
		destDriveId = driveInfo.Id
		destDriveName = driveInfo.Name
	} else {
		destFolder, err := g.driveService.Files.Get(p.DestinationFolderID).
			Fields("id, driveId").
			SupportsAllDrives(true).
			Context(ctx).
			Do()
		if err != nil {
			return nil, fmt.Errorf("destination not found: %w", err)
		}
		destDriveId = destFolder.DriveId
	}

	sourceDriveId := file.DriveId
	if sourceDriveId == "" && len(file.Parents) > 0 {
		sourceDriveId = "my-drive"
	}
	if destDriveId == "" && !destIsSharedDrive {
		destDriveId = "my-drive"
	}

	if isFolder && sourceDriveId != destDriveId {
		sourceName := "My Drive"
		destName := "My Drive"
		if sourceDriveId != "my-drive" {
			sourceName = fmt.Sprintf("Shared Drive (%s)", sourceDriveId)
		}
		if destIsSharedDrive {
			destName = destDriveName
		} else if destDriveId != "my-drive" {
			destName = fmt.Sprintf("Shared Drive (%s)", destDriveId)
		}

		return nil, fmt.Errorf(
			"cannot move folder '%s' from %s to %s. "+
				"Google Drive does not allow moving folders between drives. "+
				"You can only move files, or copy the folder contents instead.",
			file.Name, sourceName, destName,
		)
	}

	updateCall := g.driveService.Files.Update(p.FileID, &drive.File{}).
		SupportsAllDrives(true).
		AddParents(p.DestinationFolderID)

	for _, parent := range file.Parents {
		if parent != p.DestinationFolderID {
			updateCall = updateCall.RemoveParents(parent)
		}
	}

	movedFile, err := updateCall.
		Fields("id, name, parents, webViewLink, driveId").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	return movedFile, nil
}

type ShareFileParams struct {
	FileID         string `json:"file_id"`
	Role           string `json:"role"`
	Type           string `json:"type"`
	EmailAddress   string `json:"email_address,omitempty"`
	Domain         string `json:"domain,omitempty"`
	AllowDiscovery bool   `json:"allow_file_discovery,omitempty"`
}

func (g *GoogleDriveIntegration) ShareFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ShareFileParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for share file: %w", err)
	}
	if p.FileID == "" || p.Role == "" || p.Type == "" {
		return nil, fmt.Errorf("file ID, role or type is empty")
	}

	permission := &drive.Permission{
		Role: p.Role,
		Type: p.Type,
	}
	if p.Type == "user" || p.Type == "group" {
		if p.EmailAddress == "" {
			return nil, fmt.Errorf("EmailAddress is required for type '%s'", p.Type)
		}
		permission.EmailAddress = p.EmailAddress
	} else if p.Type == "domain" {
		if p.Domain == "" {
			return nil, fmt.Errorf("Domain is required for type 'domain'")
		}
		permission.Domain = p.Domain
	} else if p.Type == "anyone" {
		permission.AllowFileDiscovery = p.AllowDiscovery
	}

	createdPermission, err := g.driveService.Permissions.Create(p.FileID, permission).
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to share file: %w", err)
	}
	return createdPermission, nil
}

type UpdateFileParams struct {
	FileID      string           `json:"file_id"`
	NewFileName string           `json:"new_file_name,omitempty"`
	MimeType    string           `json:"mime_type,omitempty"`
	FileContent *domain.FileItem `json:"file_content,omitempty"`
}

func (g *GoogleDriveIntegration) UpdateFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p UpdateFileParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for update file: %w", err)
	}
	if p.FileID == "" {
		return nil, fmt.Errorf("file ID is empty")
	}

	currentFile, err := g.driveService.Files.Get(p.FileID).
		Fields("id, name, mimeType").
		SupportsAllDrives(true).
		SupportsTeamDrives(true).
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get current file info: %w", err)
	}

	fileMetadata := &drive.File{}
	hasMetadataUpdate := false
	if p.NewFileName != "" {
		fileMetadata.Name = p.NewFileName
		hasMetadataUpdate = true
	}
	if p.MimeType != "" {
		if strings.Contains(currentFile.MimeType, "vnd.google-apps") {
		} else if strings.Contains(p.MimeType, "vnd.google-apps") {
		} else {
			fileMetadata.MimeType = p.MimeType
			hasMetadataUpdate = true
		}
	}

	var updatedFile *drive.File

	if p.FileContent != nil && p.FileContent.ObjectKey != "" {
		executionFile, err := g.executorStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
			WorkspaceID: g.workspaceID,
			UploadID:    p.FileContent.FileID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get file content from object storage: %w", err)
		}

		updatedFile, err = g.driveService.Files.Update(p.FileID, fileMetadata).
			Media(executionFile.Reader).
			Fields("id, name, mimeType, webViewLink, parents, createdTime, modifiedTime, size").
			SupportsAllDrives(true).
			SupportsTeamDrives(true).
			Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to update file content: %w", err)
		}
	} else if hasMetadataUpdate {
		updatedFile, err = g.driveService.Files.Update(p.FileID, fileMetadata).
			Fields("id, name, mimeType, webViewLink, parents, createdTime, modifiedTime, size").
			SupportsAllDrives(true).
			SupportsTeamDrives(true).
			Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to update file metadata: %w", err)
		}
	} else {
		existingFile, err := g.driveService.Files.Get(p.FileID).
			Fields("id, name, mimeType, webViewLink").
			SupportsAllDrives(true).
			SupportsTeamDrives(true).
			Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get existing file info: %w", err)
		}
		updatedFile = existingFile
	}
	return updatedFile, nil
}

type SearchFilesAndFoldersParams struct {
	NameContains string `json:"name_contains"`
	PageSize     int64  `json:"page_size,omitempty"`
	PageToken    string `json:"page_token,omitempty"`
}

func (g *GoogleDriveIntegration) SearchFilesAndFolders(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p SearchFilesAndFoldersParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for search files and folders: %w", err)
	}

	if p.NameContains == "" {
		return []domain.Item{&drive.FileList{Files: []*drive.File{}}}, nil
	}

	escapedQuery := strings.ReplaceAll(p.NameContains, "'", "\\'")
	query := fmt.Sprintf("name contains '%s' and trashed = false", escapedQuery)

	filesList, err := g.driveService.Files.List().
		Q(query).
		Fields("files(id, name, mimeType, webViewLink, createdTime, modifiedTime, size, parents)").
		PageSize(p.PageSize).
		IncludeItemsFromAllDrives(true).
		SupportsAllDrives(true).
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to search files and folders with query '%s': %w", p.NameContains, err)
	}

	if len(filesList.Files) == 0 {
		return nil, nil
	}
	return filesList, nil
}

type CreateFolderParams struct {
	FolderName     string `json:"folder_name"`
	ParentFolderID string `json:"parent_folder_id,omitempty"`
}

func (g *GoogleDriveIntegration) CreateFolder(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p CreateFolderParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for create folder: %w", err)
	}
	if p.FolderName == "" {
		return nil, fmt.Errorf("folder name is empty")
	}

	folderMetadata := &drive.File{
		Name:     p.FolderName,
		MimeType: "application/vnd.google-apps.folder",
	}
	if p.ParentFolderID != "" {
		folderMetadata.Parents = []string{p.ParentFolderID}
	}

	createdFolder, err := g.driveService.Files.Create(folderMetadata).
		Fields("id, name, webViewLink, parents").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}
	return createdFolder, nil
}

type DeleteFolderParams struct {
	FolderID string `json:"folder_id"`
}

func (g *GoogleDriveIntegration) DeleteFolder(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p DeleteFolderParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for delete folder: %w", err)
	}
	if p.FolderID == "" {
		return nil, fmt.Errorf("folder ID is empty")
	}

	folderInfo, err := g.driveService.Files.Get(p.FolderID).
		Fields("id, name, mimeType, trashed").
		SupportsAllDrives(true).
		SupportsTeamDrives(true).
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get folder info before deletion: %w", err)
	}

	err = g.driveService.Files.Delete(p.FolderID).
		SupportsAllDrives(true).
		SupportsTeamDrives(true).
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to delete folder: %w", err)
	}
	return folderInfo, nil
}

type ShareFolderParams struct {
	FolderID       string `json:"folder_id"`
	Role           string `json:"role"`
	Type           string `json:"type"`
	EmailAddress   string `json:"email_address,omitempty"`
	Domain         string `json:"domain,omitempty"`
	AllowDiscovery bool   `json:"allow_file_discovery,omitempty"`
}

func (g *GoogleDriveIntegration) ShareFolder(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ShareFolderParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for share folder: %w", err)
	}
	if p.FolderID == "" || p.Role == "" || p.Type == "" {
		return nil, fmt.Errorf("folder ID, role or type is empty")
	}

	if p.FolderID == "root" {
		return nil, fmt.Errorf("cannot share My Drive root folder: Google Drive does not allow sharing the root folder")
	}

	driveInfo, driveErr := g.driveService.Drives.Get(p.FolderID).
		Fields("id, name").
		Context(ctx).
		Do()

	if driveErr == nil {
		return g.ShareSharedDrive(ctx, input, item, driveInfo.Id)
	}

	folderInfo, err := g.driveService.Files.Get(p.FolderID).
		Fields("id, name, driveId, parents").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("neither folder nor shared drive found with ID '%s': %w", p.FolderID, err)
	}

	if folderInfo.DriveId != "" {
		driveInfo, err := g.driveService.Drives.Get(folderInfo.DriveId).
			Fields("id, name").
			Context(ctx).
			Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get shared drive info: %w", err)
		}

		return nil, fmt.Errorf(
			"cannot share folder '%s' because it's inside Shared Drive '%s'. "+
				"Folders in Shared Drives cannot be individually shared. "+
				"To grant access, share the Shared Drive itself or share individual files within the folder.",
			folderInfo.Name,
			driveInfo.Name,
		)
	}

	permission := &drive.Permission{
		Role: p.Role,
		Type: p.Type,
	}

	switch p.Type {
	case "user", "group":
		if p.EmailAddress == "" {
			return nil, fmt.Errorf("email address is required for type '%s'", p.Type)
		}
		permission.EmailAddress = p.EmailAddress
	case "domain":
		if p.Domain == "" {
			return nil, fmt.Errorf("domain is required for type 'domain'")
		}
		permission.Domain = p.Domain
	case "anyone":
		permission.AllowFileDiscovery = p.AllowDiscovery
	}

	createdPermission, err := g.driveService.Permissions.Create(p.FolderID, permission).
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to share folder: %w", err)
	}

	return createdPermission, nil
}

type CreateSharedDriveParams struct {
	RequestID string `json:"request_id"`
	Name      string `json:"name"`
}

func (g *GoogleDriveIntegration) CreateSharedDrive(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p CreateSharedDriveParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for create shared drive: %w", err)
	}
	if p.RequestID == "" || p.Name == "" {
		return nil, fmt.Errorf("request ID or name is empty")
	}

	sharedDrive := &drive.Drive{Name: p.Name}
	createdDrive, err := g.driveService.Drives.Create(p.RequestID, sharedDrive).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create shared drive: %w", err)
	}
	return createdDrive, nil
}

type DeleteSharedDriveParams struct {
	SharedDriveID string `json:"shared_drive_id"`
}

func (g *GoogleDriveIntegration) DeleteSharedDrive(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p DeleteSharedDriveParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for delete shared drive: %w", err)
	}
	if p.SharedDriveID == "" {
		return nil, fmt.Errorf("shared drive ID is empty")
	}

	driveInfo, err := g.driveService.Drives.Get(p.SharedDriveID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get shared drive info before deletion: %w", err)
	}

	err = g.driveService.Drives.Delete(p.SharedDriveID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to delete shared drive: %w", err)
	}
	return driveInfo, nil
}

type GetSharedDriveParams struct {
	SharedDriveID string `json:"shared_drive_id"`
}

func (g *GoogleDriveIntegration) GetSharedDrive(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p GetSharedDriveParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for get shared drive: %w", err)
	}
	if p.SharedDriveID == "" {
		return nil, fmt.Errorf("shared drive ID is empty")
	}

	driveItem, err := g.driveService.Drives.Get(p.SharedDriveID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get shared drive: %w", err)
	}
	return driveItem, nil
}

type ListSharedDrivesParams struct {
	PageSize  int64  `json:"page_size,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

func (g *GoogleDriveIntegration) ListSharedDrives(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ListSharedDrivesParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for list shared drives: %w", err)
	}

	listCall := g.driveService.Drives.List()
	if p.PageSize > 0 {
		listCall = listCall.PageSize(p.PageSize)
	}
	if p.PageToken != "" {
		listCall = listCall.PageToken(p.PageToken)
	}

	driveList, err := listCall.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list shared drives: %w", err)
	}
	if len(driveList.Drives) == 0 {
		return nil, nil
	}
	return driveList, nil
}

type UpdateSharedDriveParams struct {
	SharedDriveID string `json:"shared_drive_id"`
	Name          string `json:"name,omitempty"`
	ThemeID       string `json:"theme_id,omitempty"`
}

func (g *GoogleDriveIntegration) UpdateSharedDrive(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p UpdateSharedDriveParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters for update shared drive: %w", err)
	}
	if p.SharedDriveID == "" {
		return nil, fmt.Errorf("shared drive ID is empty")
	}

	driveUpdate := &drive.Drive{}
	hasUpdate := false
	if p.Name != "" {
		driveUpdate.Name = p.Name
		hasUpdate = true
	}
	if p.ThemeID != "" {
		driveUpdate.ThemeId = p.ThemeID
		hasUpdate = true
	}

	if !hasUpdate {
		currentDrive, err := g.driveService.Drives.Get(p.SharedDriveID).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get current shared drive info: %w", err)
		}
		return currentDrive, nil
	}

	updatedDrive, err := g.driveService.Drives.Update(p.SharedDriveID, driveUpdate).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to update shared drive: %w", err)
	}
	return updatedDrive, nil
}

func (g *GoogleDriveIntegration) ShareSharedDrive(ctx context.Context, input domain.IntegrationInput, item domain.Item, driveId string) (domain.Item, error) {
	var p ShareFolderParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	// Map roles for Shared Drives
	// Shared Drives use different role names than regular files/folders
	driveRole := p.Role
	switch p.Role {
	case "reader":
		driveRole = "reader"
	case "commenter":
		driveRole = "commenter"
	case "writer":
		driveRole = "writer"
	case "fileOrganizer":
		driveRole = "fileOrganizer"
	case "organizer":
		driveRole = "organizer"
	default:
		driveRole = p.Role
	}

	permission := &drive.Permission{
		Role: driveRole,
		Type: p.Type,
	}

	switch p.Type {
	case "user", "group":
		if p.EmailAddress == "" {
			return nil, fmt.Errorf("email address is required for type '%s'", p.Type)
		}
		permission.EmailAddress = p.EmailAddress
	case "domain":
		if p.Domain == "" {
			return nil, fmt.Errorf("domain is required for type 'domain'")
		}
		permission.Domain = p.Domain
	default:
		return nil, fmt.Errorf("Shared Drives can only be shared with specific users, groups, or domains")
	}

	createdPermission, err := g.driveService.Permissions.Create(driveId, permission).
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to share Shared Drive '%s': %w", driveId, err)
	}

	return createdPermission, nil
}

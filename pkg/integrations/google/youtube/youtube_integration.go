package youtube

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type YoutubeIntegrationCreator struct {
	credentialGetter       domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
}

func NewYoutubeIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &YoutubeIntegrationCreator{
		credentialGetter:       managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}
}

func (c *YoutubeIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	integration, err := NewYoutubeIntegration(ctx, YoutubeIntegrationDependencies{
		WorkspaceID:            p.WorkspaceID,
		CredentialGetter:       c.credentialGetter,
		ParameterBinder:        c.binder,
		CredentialID:           p.CredentialID,
		ExecutorStorageManager: c.executorStorageManager,
	})
	if err != nil {
		return nil, err
	}
	return integration, nil
}

type YoutubeIntegration struct {
	workspaceID            string
	binder                 domain.IntegrationParameterBinder
	youtubeService         *youtube.Service
	executorStorageManager domain.ExecutorStorageManager

	actionManager *domain.IntegrationActionManager
}

type YoutubeIntegrationDependencies struct {
	WorkspaceID            string
	CredentialGetter       domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	ParameterBinder        domain.IntegrationParameterBinder
	CredentialID           string
	ExecutorStorageManager domain.ExecutorStorageManager
}

func NewYoutubeIntegration(ctx context.Context, deps YoutubeIntegrationDependencies) (*YoutubeIntegration, error) {
	credential, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	token := &oauth2.Token{
		AccessToken: credential.AccessToken,
		TokenType:   "Bearer",
	}

	youtubeService, err := youtube.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(token)))
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	integration := &YoutubeIntegration{
		workspaceID:            deps.WorkspaceID,
		binder:                 deps.ParameterBinder,
		youtubeService:         youtubeService,
		executorStorageManager: deps.ExecutorStorageManager,
		actionManager:          domain.NewIntegrationActionManager(),
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_UploadVideo, integration.UploadVideo).
		AddPerItem(IntegrationActionType_GetVideoBrief, integration.GetVideoBrief).
		AddPerItem(IntegrationActionType_GetVideoBriefs, integration.GetVideoBriefs).
		AddPerItem(IntegrationActionType_DeleteVideo, integration.DeleteVideo)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *YoutubeIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type UploadVideoParams struct {
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	Video         domain.FileItem `json:"video"`
	Tags          []string        `json:"tags"`
	PrivacyStatus string          `json:"privacy_status"`
}

type UploadVideoOutput struct {
	VideoID string `json:"video_id"`
	Status  string `json:"status"`
}

func (i *YoutubeIntegration) UploadVideo(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var uploadParams UploadVideoParams

	if err := i.binder.BindToStruct(ctx, item, &uploadParams, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind to struct: %w", err)
	}

	call := i.youtubeService.Videos.Insert([]string{"snippet", "status"}, &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       uploadParams.Title,
			Description: uploadParams.Description,
			Tags:        uploadParams.Tags,
		},
		Status: &youtube.VideoStatus{
			PrivacyStatus: uploadParams.PrivacyStatus,
		},
	})

	executionFile, err := i.executorStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
		WorkspaceID: i.workspaceID,
		UploadID:    uploadParams.Video.FileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get execution file: %w", err)
	}

	call = call.Media(executionFile.Reader)

	video, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to upload video: %w", err)
	}

	videoID := video.Id

	outputItem := UploadVideoOutput{
		VideoID: videoID,
		Status:  "uploaded",
	}

	return outputItem, nil
}

type DeleteVideoParams struct {
	VideoID string `json:"video_id"`
}

func (i *YoutubeIntegration) DeleteVideo(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var deleteVideoParams DeleteVideoParams

	if err := i.binder.BindToStruct(ctx, item, &deleteVideoParams, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind to struct: %w", err)
	}

	call := i.youtubeService.Videos.Delete(deleteVideoParams.VideoID)

	err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to delete video: %w", err)
	}

	return nil, nil
}

type GetVideoBriefParams struct {
	VideoID string `json:"video_id"`
}

func (i *YoutubeIntegration) GetVideoBrief(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var getVideoBriefParams GetVideoBriefParams

	if err := i.binder.BindToStruct(ctx, item, &getVideoBriefParams, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind to struct: %w", err)
	}

	video, err := i.youtubeService.Videos.List([]string{"snippet", "status"}).Id(getVideoBriefParams.VideoID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get video brief: %w", err)
	}

	return video, nil
}

type GetVideoBriefsParams struct {
	Limit             int    `json:"limit"`
	Query             string `json:"query"`
	Order             string `json:"order"`
	Type              string `json:"type"`
	VideoDuration     string `json:"videoDuration"`
	PublishedAfter    string `json:"publishedAfter"`
	PublishedBefore   string `json:"publishedBefore"`
	RegionCode        string `json:"regionCode"`
	RelevanceLanguage string `json:"relevanceLanguage"`
	ChannelID         string `json:"channelId"`
}

func (i *YoutubeIntegration) GetVideoBriefs(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var getVideoBriefsParams GetVideoBriefsParams

	if err := i.binder.BindToStruct(ctx, item, &getVideoBriefsParams, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind to struct: %w", err)
	}

	searchCall := i.youtubeService.Search.List([]string{"snippet"}).
		MaxResults(int64(getVideoBriefsParams.Limit))

	// Add query if provided
	if getVideoBriefsParams.Query != "" {
		searchCall = searchCall.Q(getVideoBriefsParams.Query)
	}

	// Add order if provided
	if getVideoBriefsParams.Order != "" {
		searchCall = searchCall.Order(getVideoBriefsParams.Order)
	}

	// Add type if provided
	if getVideoBriefsParams.Type != "" {
		searchCall = searchCall.Type(getVideoBriefsParams.Type)
	}

	// Add video duration if provided (only valid when type includes video)
	if getVideoBriefsParams.VideoDuration != "" && getVideoBriefsParams.VideoDuration != "any" {
		searchCall = searchCall.VideoDuration(getVideoBriefsParams.VideoDuration)
	}

	// Add published after date if provided
	if getVideoBriefsParams.PublishedAfter != "" {
		searchCall = searchCall.PublishedAfter(getVideoBriefsParams.PublishedAfter)
	}

	// Add published before date if provided
	if getVideoBriefsParams.PublishedBefore != "" {
		searchCall = searchCall.PublishedBefore(getVideoBriefsParams.PublishedBefore)
	}

	// Add region code if provided
	if getVideoBriefsParams.RegionCode != "" {
		searchCall = searchCall.RegionCode(getVideoBriefsParams.RegionCode)
	}

	// Add relevance language if provided
	if getVideoBriefsParams.RelevanceLanguage != "" {
		searchCall = searchCall.RelevanceLanguage(getVideoBriefsParams.RelevanceLanguage)
	}

	// Add channel ID if provided
	if getVideoBriefsParams.ChannelID != "" {
		searchCall = searchCall.ChannelId(getVideoBriefsParams.ChannelID)
	}

	search, err := searchCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get video briefs: %w", err)
	}

	return search, nil
}

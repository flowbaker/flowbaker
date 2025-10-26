package teams

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
	absser "github.com/microsoft/kiota-abstractions-go/serialization"
	auth "github.com/microsoft/kiota-authentication-azure-go"
	jsonser "github.com/microsoft/kiota-serialization-json-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
	"github.com/rs/zerolog/log"
)

type TeamsIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewTeamsIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &TeamsIntegrationCreator{
		binder:           deps.ParameterBinder,
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *TeamsIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewTeamsIntegration(ctx, TeamsIntegrationDependencies{
		CredentialID:     p.CredentialID,
		ParameterBinder:  c.binder,
		CredentialGetter: c.credentialGetter,
	})
}

type TeamsIntegration struct {
	graphClient *msgraphsdk.GraphServiceClient

	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]

	actionManager *domain.IntegrationActionManager
	peekFuncs     map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)

	accessToken string
	tokenExpiry time.Time
}

type TeamsIntegrationDependencies struct {
	CredentialID string

	ParameterBinder  domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewTeamsIntegration(ctx context.Context, deps TeamsIntegrationDependencies) (*TeamsIntegration, error) {
	integration := &TeamsIntegration{
		binder:           deps.ParameterBinder,
		credentialGetter: deps.CredentialGetter,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(TeamsActionType_SendChannelMessage, integration.SendChannelMessage).
		AddPerItem(TeamsActionType_SendChatMessage, integration.SendChatMessage).
		AddPerItem(TeamsActionType_CreateChannel, integration.CreateChannel).
		AddPerItem(TeamsActionType_CreateTeam, integration.CreateTeam).
		AddPerItem(TeamsActionType_DeleteChannel, integration.DeleteChannel).
		AddPerItem(TeamsActionType_GetChannel, integration.GetChannel).
		AddPerItemMulti(TeamsActionType_GetManyChannels, integration.GetManyChannels).
		AddPerItem(TeamsActionType_UpdateChannel, integration.UpdateChannel).
		AddPerItemMulti(TeamsActionType_GetChannelMessages, integration.GetChannelMessages).
		AddPerItem(TeamsActionType_GetChatMessage, integration.GetChatMessage).
		AddPerItemMulti(TeamsActionType_GetManyChatMessages, integration.GetManyChatMessages)

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		TeamsPeekable_Channels: integration.PeekChannels,
		TeamsPeekable_Chats:    integration.PeekChats,
		TeamsPeekable_Teams:    integration.PeekTeams,
	}

	integration.actionManager = actionManager
	integration.peekFuncs = peekFuncs

	if deps.CredentialID == "" {
		return nil, fmt.Errorf("credential ID is required for Teams integration")
	}

	oauthAccount, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get decrypted Teams OAuth credential: %w", err)
	}

	integration.accessToken = oauthAccount.AccessToken
	integration.tokenExpiry = oauthAccount.Expiry

	credential := &TeamsTokenCredential{accessToken: oauthAccount.AccessToken}
	authProvider, err := auth.NewAzureIdentityAuthenticationProvider(credential)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth provider: %w", err)
	}

	adapter, err := msgraphsdk.NewGraphRequestAdapter(authProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create Graph request adapter: %w", err)
	}

	integration.graphClient = msgraphsdk.NewGraphServiceClient(adapter)

	return integration, nil
}

func (i *TeamsIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *TeamsIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function %s not found for Teams integration", params.PeekableType)
	}
	return peekFunc(ctx, params)
}

func (i *TeamsIntegration) SendChannelMessage(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := SendChannelMessageParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ChannelID == "" {
		return nil, fmt.Errorf("channel_id is required")
	}
	if params.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	channelParser := &TeamsChannelIdParser{}
	teamID, channelID, err := channelParser.ParseChannelID(params.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse channel_id: %w", err)
	}

	contentType := models.HTML_BODYTYPE
	if params.ContentType != nil && *params.ContentType == "text" {
		contentType = models.TEXT_BODYTYPE
	}

	requestBody := models.NewChatMessage()
	body := models.NewItemBody()
	body.SetContent(&params.Message)
	body.SetContentType(&contentType)
	requestBody.SetBody(body)

	result, err := i.graphClient.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().Post(ctx, requestBody, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)
		return nil, fmt.Errorf("failed to send message to Teams channel: [%s] %s", errDetails.Code, errDetails.Message)
	}

	jsonParser := &JsonParser{}
	return jsonParser.ConvertToRawJSON(result)
}

func (i *TeamsIntegration) SendChatMessage(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := SendChatMessageParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ChatID == "" {
		return nil, fmt.Errorf("chat_id is required")
	}
	if params.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	contentType := models.HTML_BODYTYPE
	if params.ContentType != nil && *params.ContentType == "text" {
		contentType = models.TEXT_BODYTYPE
	}

	requestBody := models.NewChatMessage()
	body := models.NewItemBody()
	body.SetContent(&params.Message)
	body.SetContentType(&contentType)
	requestBody.SetBody(body)

	result, err := i.graphClient.Chats().ByChatId(params.ChatID).Messages().Post(ctx, requestBody, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)
		return nil, fmt.Errorf("failed to send message to Teams chat: [%s] %s", errDetails.Code, errDetails.Message)
	}

	jsonParser := &JsonParser{}
	return jsonParser.ConvertToRawJSON(result)
}

func (i *TeamsIntegration) CreateChannel(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreateChannelParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.TeamID == "" {
		return nil, fmt.Errorf("team_id is required")
	}
	if params.ChannelName == "" {
		return nil, fmt.Errorf("channel_name is required")
	}

	membershipType := models.STANDARD_CHANNELMEMBERSHIPTYPE
	if params.ChannelType != nil && *params.ChannelType == "private" {
		membershipType = models.PRIVATE_CHANNELMEMBERSHIPTYPE
	}

	requestBody := models.NewChannel()
	requestBody.SetDisplayName(&params.ChannelName)
	requestBody.SetMembershipType(&membershipType)

	if params.ChannelDescription != nil && *params.ChannelDescription != "" {
		requestBody.SetDescription(params.ChannelDescription)
	}

	result, err := i.graphClient.Teams().ByTeamId(params.TeamID).Channels().Post(ctx, requestBody, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)

		return nil, fmt.Errorf("failed to create Teams channel: [%s] %s", errDetails.Code, errDetails.Message)
	}

	jsonParser := &JsonParser{}
	return jsonParser.ConvertToRawJSON(result)
}

func (i *TeamsIntegration) CreateTeam(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreateTeamParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.TeamName == "" {
		return nil, fmt.Errorf("team_name is required")
	}

	visibility := "Private"
	if params.TeamVisibility != nil && *params.TeamVisibility == "public" {
		visibility = "Public"
	}

	groupRequestBody := models.NewGroup()
	groupRequestBody.SetDisplayName(&params.TeamName)
	groupRequestBody.SetMailEnabled(func() *bool { b := true; return &b }())
	groupRequestBody.SetMailNickname(func() *string {
		nickname := strings.ToLower(strings.ReplaceAll(params.TeamName, " ", "-"))
		return &nickname
	}())
	groupRequestBody.SetSecurityEnabled(func() *bool { b := false; return &b }())
	groupRequestBody.SetVisibility(&visibility)

	if params.TeamDescription != nil && *params.TeamDescription != "" {
		groupRequestBody.SetDescription(params.TeamDescription)
	}

	groupTypes := []string{"Unified"}
	groupRequestBody.SetGroupTypes(groupTypes)

	group, err := i.graphClient.Groups().Post(ctx, groupRequestBody, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)

		return nil, fmt.Errorf("failed to create Microsoft 365 Group: [%s] %s", errDetails.Code, errDetails.Message)
	}

	if group.GetId() == nil {
		return nil, fmt.Errorf("created group has no ID")
	}

	groupID := *group.GetId()

	teamRequestBody := models.NewTeam()

	messagingSettings := models.NewTeamMessagingSettings()
	allowUserEditMessages := true
	allowUserDeleteMessages := true
	messagingSettings.SetAllowUserEditMessages(&allowUserEditMessages)
	messagingSettings.SetAllowUserDeleteMessages(&allowUserDeleteMessages)
	teamRequestBody.SetMessagingSettings(messagingSettings)

	funSettings := models.NewTeamFunSettings()
	allowGiphy := true
	funSettings.SetAllowGiphy(&allowGiphy)
	teamRequestBody.SetFunSettings(funSettings)

	team, err := i.graphClient.Groups().ByGroupId(groupID).Team().Put(ctx, teamRequestBody, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)

		return nil, fmt.Errorf("failed to convert group to Team: [%s] %s", errDetails.Code, errDetails.Message)
	}

	jsonParser := &JsonParser{}
	return jsonParser.ConvertToRawJSON(team)
}

func (i *TeamsIntegration) DeleteChannel(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := DeleteChannelParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ChannelID == "" {
		return nil, fmt.Errorf("channel_id is required")
	}

	channelParser := &TeamsChannelIdParser{}
	teamID, channelID, err := channelParser.ParseChannelID(params.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse channel_id: %w", err)
	}

	err = i.graphClient.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Delete(ctx, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)

		log.Error().
			Err(err).
			Str("team_id", teamID).
			Str("channel_id", channelID).
			Str("error_code", errDetails.Code).
			Str("error_message", errDetails.Message).
			Str("error_details", errDetails.Details).
			Msg("Failed to delete Teams channel")

		return nil, fmt.Errorf("failed to delete Teams channel: [%s] %s", errDetails.Code, errDetails.Message)
	}

	return map[string]interface{}{
		"success":    true,
		"team_id":    teamID,
		"channel_id": channelID,
	}, nil
}

func (i *TeamsIntegration) GetChannel(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetChannelParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.TeamID == "" {
		return nil, fmt.Errorf("team_id is required")
	}
	if params.ChannelID == "" {
		return nil, fmt.Errorf("channel_id is required")
	}

	result, err := i.graphClient.Teams().ByTeamId(params.TeamID).Channels().ByChannelId(params.ChannelID).Get(ctx, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)

		log.Error().
			Err(err).
			Str("team_id", params.TeamID).
			Str("channel_id", params.ChannelID).
			Str("error_code", errDetails.Code).
			Str("error_message", errDetails.Message).
			Str("error_details", errDetails.Details).
			Msg("Failed to get Teams channel")

		return nil, fmt.Errorf("failed to get Teams channel: [%s] %s", errDetails.Code, errDetails.Message)
	}

	jsonParser := &JsonParser{}
	return jsonParser.ConvertToRawJSON(result)
}

func (i *TeamsIntegration) GetManyChannels(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := GetManyChannelsParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.TeamID == "" {
		return nil, fmt.Errorf("team_id is required")
	}

	result, err := i.graphClient.Teams().ByTeamId(params.TeamID).Channels().Get(ctx, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)

		log.Error().
			Err(err).
			Str("team_id", params.TeamID).
			Str("error_code", errDetails.Code).
			Str("error_message", errDetails.Message).
			Str("error_details", errDetails.Details).
			Msg("Failed to get Teams channels")

		return nil, fmt.Errorf("failed to get Teams channels: [%s] %s", errDetails.Code, errDetails.Message)
	}

	if result != nil && result.GetValue() != nil {
		channels := result.GetValue()
		items := make([]domain.Item, 0, len(channels))
		jsonParser := &JsonParser{}
		for _, channel := range channels {
			channelMap, err := jsonParser.ConvertToRawJSON(channel)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to convert channel to JSON")
				continue
			}
			items = append(items, channelMap)
		}
		return items, nil
	}

	return []domain.Item{}, nil
}

func (i *TeamsIntegration) UpdateChannel(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := UpdateChannelParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.TeamID == "" {
		return nil, fmt.Errorf("team_id is required")
	}
	if params.ChannelID == "" {
		return nil, fmt.Errorf("channel_id is required")
	}

	requestBody := models.NewChannel()
	if params.ChannelName != nil && *params.ChannelName != "" {
		requestBody.SetDisplayName(params.ChannelName)
	}
	if params.ChannelDescription != nil {
		requestBody.SetDescription(params.ChannelDescription)
	}

	result, err := i.graphClient.Teams().ByTeamId(params.TeamID).Channels().ByChannelId(params.ChannelID).Patch(ctx, requestBody, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)

		log.Error().
			Err(err).
			Str("team_id", params.TeamID).
			Str("channel_id", params.ChannelID).
			Str("error_code", errDetails.Code).
			Str("error_message", errDetails.Message).
			Str("error_details", errDetails.Details).
			Msg("Failed to update Teams channel")

		return nil, fmt.Errorf("failed to update Teams channel: [%s] %s", errDetails.Code, errDetails.Message)
	}

	jsonParser := &JsonParser{}
	return jsonParser.ConvertToRawJSON(result)
}

func (i *TeamsIntegration) GetChannelMessages(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := GetChannelMessagesParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ChannelID == "" {
		return nil, fmt.Errorf("channel_id is required")
	}

	channelParser := &TeamsChannelIdParser{}
	teamID, channelID, err := channelParser.ParseChannelID(params.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse channel_id: %w", err)
	}

	result, err := i.graphClient.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().Get(ctx, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)

		log.Error().
			Err(err).
			Str("team_id", teamID).
			Str("channel_id", channelID).
			Str("error_code", errDetails.Code).
			Str("error_message", errDetails.Message).
			Str("error_details", errDetails.Details).
			Msg("Failed to get channel messages")

		return nil, fmt.Errorf("failed to get channel messages: [%s] %s", errDetails.Code, errDetails.Message)
	}

	if result != nil && result.GetValue() != nil {
		messages := result.GetValue()
		items := make([]domain.Item, 0, len(messages))
		jsonParser := &JsonParser{}
		for _, message := range messages {
			messageMap, err := jsonParser.ConvertToRawJSON(message)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to convert message to JSON")
				continue
			}
			items = append(items, messageMap)
		}
		return items, nil
	}

	return []domain.Item{}, nil
}

func (i *TeamsIntegration) GetChatMessage(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetChatMessageParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ChatID == "" {
		return nil, fmt.Errorf("chat_id is required")
	}
	if params.MessageID == "" {
		return nil, fmt.Errorf("message_id is required")
	}

	result, err := i.graphClient.Chats().ByChatId(params.ChatID).Messages().ByChatMessageId(params.MessageID).Get(ctx, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)

		log.Error().
			Err(err).
			Str("chat_id", params.ChatID).
			Str("message_id", params.MessageID).
			Str("error_code", errDetails.Code).
			Str("error_message", errDetails.Message).
			Str("error_details", errDetails.Details).
			Msg("Failed to get chat message")

		return nil, fmt.Errorf("failed to get chat message: [%s] %s", errDetails.Code, errDetails.Message)
	}

	jsonParser := &JsonParser{}
	return jsonParser.ConvertToRawJSON(result)
}

func (i *TeamsIntegration) GetManyChatMessages(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := GetManyChatMessagesParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ChatID == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	result, err := i.graphClient.Chats().ByChatId(params.ChatID).Messages().Get(ctx, nil)
	if err != nil {
		errorParser := &ODataErrorParser{}
		errDetails := errorParser.ParseError(err)

		log.Error().
			Err(err).
			Str("chat_id", params.ChatID).
			Str("error_code", errDetails.Code).
			Str("error_message", errDetails.Message).
			Str("error_details", errDetails.Details).
			Msg("Failed to get chat messages")

		return nil, fmt.Errorf("failed to get chat messages: [%s] %s", errDetails.Code, errDetails.Message)
	}

	if result != nil && result.GetValue() != nil {
		messages := result.GetValue()
		items := make([]domain.Item, 0, len(messages))
		jsonParser := &JsonParser{}
		for _, message := range messages {
			messageMap, err := jsonParser.ConvertToRawJSON(message)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to convert message to JSON")
				continue
			}
			items = append(items, messageMap)
		}
		return items, nil
	}

	return []domain.Item{}, nil
}

func (i *TeamsIntegration) PeekChannels(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	var results []domain.PeekResultItem

	teamsResult, err := i.graphClient.Me().JoinedTeams().Get(ctx, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get joined teams")
		return domain.PeekResult{}, fmt.Errorf("failed to get joined teams: %w. Make sure the user has a Teams license and necessary permissions are granted", err)
	}

	if teamsResult == nil || teamsResult.GetValue() == nil {
		log.Info().Msg("No teams found for user")
		return domain.PeekResult{Result: results}, nil
	}

	for _, team := range teamsResult.GetValue() {
		if team.GetId() == nil || team.GetDisplayName() == nil {
			continue
		}

		teamID := *team.GetId()
		teamName := *team.GetDisplayName()

		channelsResult, err := i.graphClient.Teams().ByTeamId(teamID).Channels().Get(ctx, nil)
		if err != nil {
			log.Warn().Err(err).Str("team_id", teamID).Str("team_name", teamName).Msg("Failed to get channels for team")
			continue
		}

		if channelsResult != nil && channelsResult.GetValue() != nil {

			for _, channel := range channelsResult.GetValue() {
				if channel.GetId() == nil || channel.GetDisplayName() == nil {
					continue
				}

				channelID := *channel.GetId()
				channelName := *channel.GetDisplayName()
				channelKey := fmt.Sprintf("%s:%s", teamID, channelID)

				results = append(results, domain.PeekResultItem{
					Key:     channelKey,
					Value:   channelKey,
					Content: fmt.Sprintf("%s / %s", teamName, channelName),
				})
			}
		}
	}

	return domain.PeekResult{Result: results}, nil
}

func (i *TeamsIntegration) PeekTeams(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	var results []domain.PeekResultItem

	teamsResult, err := i.graphClient.Me().JoinedTeams().Get(ctx, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get joined teams")
		return domain.PeekResult{}, fmt.Errorf("failed to get joined teams: %w. Make sure the user has a Teams license and necessary permissions are granted", err)
	}

	if teamsResult == nil || teamsResult.GetValue() == nil {
		log.Info().Msg("No teams found for user")
		return domain.PeekResult{Result: results}, nil
	}

	for _, team := range teamsResult.GetValue() {
		if team.GetId() == nil || team.GetDisplayName() == nil {
			continue
		}

		teamID := *team.GetId()
		teamName := *team.GetDisplayName()

		results = append(results, domain.PeekResultItem{
			Key:     teamID,
			Value:   teamID,
			Content: teamName,
		})
	}

	return domain.PeekResult{Result: results}, nil
}

func (i *TeamsIntegration) PeekChats(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	var results []domain.PeekResultItem

	chatsResult, err := i.graphClient.Me().Chats().Get(ctx, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats")
		return domain.PeekResult{}, fmt.Errorf("failed to get chats: %w", err)
	}

	if chatsResult == nil || chatsResult.GetValue() == nil {
		return domain.PeekResult{Result: results}, nil
	}

	for _, chat := range chatsResult.GetValue() {
		if chat.GetId() == nil {
			continue
		}

		chatID := *chat.GetId()
		chatBuilder := &ChatNameBuilder{graphClient: i.graphClient}
		chatType := chatBuilder.GetChatType(chat)
		chatName := chatBuilder.BuildChatName(ctx, BuildChatNameParams{
			Chat:     chat,
			ChatID:   chatID,
			ChatType: chatType,
		})

		results = append(results, domain.PeekResultItem{
			Key:     chatID,
			Value:   chatID,
			Content: chatName,
		})
	}

	return domain.PeekResult{Result: results}, nil
}

type ODataErrorDetails struct {
	Code    string
	Message string
	Details string
}

type ODataErrorParser struct{}

func (p *ODataErrorParser) ParseError(err error) ODataErrorDetails {
	details := ODataErrorDetails{
		Code:    "UnknownError",
		Message: err.Error(),
		Details: "",
	}

	if odataErr, ok := err.(*odataerrors.ODataError); ok {
		mainErr := odataErr.GetErrorEscaped()
		if mainErr != nil {
			if mainErr.GetCode() != nil {
				details.Code = *mainErr.GetCode()
			}
			if mainErr.GetMessage() != nil {
				details.Message = *mainErr.GetMessage()
			}

			if innerErr := mainErr.GetInnerError(); innerErr != nil {
				details.Details = fmt.Sprintf("InnerError: %+v", innerErr)
			}
		}
	}

	return details
}

type TeamsTokenCredential struct {
	accessToken string
}

func (c *TeamsTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     c.accessToken,
		ExpiresOn: time.Now().Add(1 * time.Hour),
	}, nil
}

type SendChannelMessageParams struct {
	ChannelID   string  `json:"channel_id"`
	Message     string  `json:"message"`
	ContentType *string `json:"content_type,omitempty"`
}

type SendChatMessageParams struct {
	ChatID      string  `json:"chat_id"`
	Message     string  `json:"message"`
	ContentType *string `json:"content_type,omitempty"`
}

type CreateChannelParams struct {
	TeamID             string  `json:"team_id"`
	ChannelName        string  `json:"channel_name"`
	ChannelDescription *string `json:"channel_description,omitempty"`
	ChannelType        *string `json:"channel_type,omitempty"`
}

type CreateTeamParams struct {
	TeamName        string  `json:"team_name"`
	TeamDescription *string `json:"team_description,omitempty"`
	TeamVisibility  *string `json:"team_visibility,omitempty"`
}

type DeleteChannelParams struct {
	ChannelID string `json:"channel_id"`
}

type GetChannelParams struct {
	TeamID    string `json:"team_id"`
	ChannelID string `json:"channel_id"`
}

type GetManyChannelsParams struct {
	TeamID string `json:"team_id"`
}

type UpdateChannelParams struct {
	TeamID             string  `json:"team_id"`
	ChannelID          string  `json:"channel_id"`
	ChannelName        *string `json:"channel_name,omitempty"`
	ChannelDescription *string `json:"channel_description,omitempty"`
}

type GetChannelMessagesParams struct {
	ChannelID string `json:"channel_id"`
	Top       *int   `json:"top,omitempty"`
}

type GetChatMessageParams struct {
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
}

type GetManyChatMessagesParams struct {
	ChatID string `json:"chat_id"`
	Top    *int   `json:"top,omitempty"`
}

type TeamsChannelIdParser struct{}

func (p *TeamsChannelIdParser) ParseChannelID(channelID string) (teamID string, channelIDParsed string, err error) {
	parts := strings.SplitN(channelID, ":", 2)

	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("channel_id must be in format 'teamId:channelId'")
}

type JsonParser struct{}

func (p *JsonParser) ConvertToRawJSON(result interface{}) (map[string]interface{}, error) {
	if parsable, ok := result.(absser.Parsable); ok {
		writer := jsonser.NewJsonSerializationWriter()
		err := writer.WriteObjectValue("", parsable)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize with Kiota: %w", err)
		}

		jsonBytes, err := writer.GetSerializedContent()
		if err != nil {
			return nil, fmt.Errorf("failed to get serialized content: %w", err)
		}

		var rawMap map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &rawMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}

		return rawMap, nil
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result to JSON: %w", err)
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &rawMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	return rawMap, nil
}

type BuildChatNameParams struct {
	Chat     models.Chatable
	ChatID   string
	ChatType string
}

type ChatNameBuilder struct {
	graphClient *msgraphsdk.GraphServiceClient
}

func (b *ChatNameBuilder) GetChatType(chat models.Chatable) string {
	if chat.GetChatType() == nil {
		return "Chat"
	}

	switch *chat.GetChatType() {
	case models.ONEONONE_CHATTYPE:
		return "1:1"
	case models.GROUP_CHATTYPE:
		return "Group"
	case models.MEETING_CHATTYPE:
		return "Meeting"
	default:
		return "Chat"
	}
}

func (b *ChatNameBuilder) BuildChatName(ctx context.Context, params BuildChatNameParams) string {
	if topic := params.Chat.GetTopic(); topic != nil && *topic != "" {
		return fmt.Sprintf("[%s] %s", params.ChatType, *topic)
	}

	memberNames := b.getChatMemberNames(ctx, params.ChatID)
	if len(memberNames) == 0 {
		return fmt.Sprintf("[%s] Chat %s", params.ChatType, params.ChatID[:8])
	}

	if len(memberNames) > 3 {
		return fmt.Sprintf("[%s] %s and %d others", params.ChatType, strings.Join(memberNames[:3], ", "), len(memberNames)-3)
	}

	return fmt.Sprintf("[%s] %s", params.ChatType, strings.Join(memberNames, ", "))
}

func (b *ChatNameBuilder) getChatMemberNames(ctx context.Context, chatID string) []string {
	members, err := b.graphClient.Chats().ByChatId(chatID).Members().Get(ctx, nil)
	if err != nil || members == nil || members.GetValue() == nil {
		return nil
	}

	var memberNames []string
	for _, member := range members.GetValue() {
		if aadMember, ok := member.(interface{ GetDisplayName() *string }); ok {
			if displayName := aadMember.GetDisplayName(); displayName != nil && *displayName != "" {
				memberNames = append(memberNames, *displayName)
			}
		}
	}

	return memberNames
}

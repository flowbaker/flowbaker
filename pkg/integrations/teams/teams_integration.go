package teams

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
	auth "github.com/microsoft/kiota-authentication-azure-go"
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

// TeamsIntegration implements the domain.IntegrationExecutor interface for Microsoft Teams
type TeamsIntegration struct {
	graphClient *msgraphsdk.GraphServiceClient

	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]

	actionManager *domain.IntegrationActionManager
	peekFuncs     map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)

	// Store access token for debugging
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
		AddPerItem(TeamsActionType_SendMessage, integration.SendMessage).
		AddPerItem(TeamsActionType_CreateChannel, integration.CreateChannel).
		AddPerItem(TeamsActionType_CreateTeam, integration.CreateTeam)

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		TeamsPeekable_Channels: integration.PeekChannels,
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

	// Log token information for debugging
	tokenLength := len(oauthAccount.AccessToken)
	tokenPreview := ""
	if tokenLength > 30 {
		tokenPreview = oauthAccount.AccessToken[:20] + "..." + oauthAccount.AccessToken[tokenLength-10:]
	} else {
		tokenPreview = oauthAccount.AccessToken
	}

	log.Info().
		Str("credential_id", deps.CredentialID).
		Int("token_length", tokenLength).
		Str("token_preview", tokenPreview).
		Time("expiry", oauthAccount.Expiry).
		Msg("Teams OAuth token retrieved for integration initialization")

	// Store token info for debugging
	integration.accessToken = oauthAccount.AccessToken
	integration.tokenExpiry = oauthAccount.Expiry

	// Create a credential provider for Microsoft Graph
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

// SendMessage sends a message to a Teams channel or chat
func (i *TeamsIntegration) SendMessage(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	// Log token being used in this action for debugging
	tokenLength := len(i.accessToken)

	log.Info().
		Int("token_length", tokenLength).
		Str("token_preview", i.accessToken).
		Time("token_expiry", i.tokenExpiry).
		Bool("token_expired", i.tokenExpiry.Before(time.Now())).
		Msg("SendMessage: OAuth token being used for this action")

	params := SendMessageParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ChannelID == "" {
		return nil, fmt.Errorf("channel_id is required")
	}
	if params.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	// Determine content type
	contentType := models.HTML_BODYTYPE
	if params.ContentType != nil && *params.ContentType == "text" {
		contentType = models.TEXT_BODYTYPE
	}

	// Create the message
	requestBody := models.NewChatMessage()
	body := models.NewItemBody()
	body.SetContent(&params.Message)
	body.SetContentType(&contentType)
	requestBody.SetBody(body)

	// Check if it's a chat (starts with "chat:")
	if strings.HasPrefix(params.ChannelID, "chat:") {
		chatID := strings.TrimPrefix(params.ChannelID, "chat:")

		log.Info().Str("chat_id", chatID).Str("message_preview", func() string {
			if len(params.Message) > 50 {
				return params.Message[:50] + "..."
			}
			return params.Message
		}()).Msg("Sending message to Teams chat")

		// Send message to chat
		result, err := i.graphClient.Chats().ByChatId(chatID).Messages().Post(ctx, requestBody, nil)
		if err != nil {
			// Extract detailed error information
			errorCode, errorMessage, errorDetails := extractODataErrorDetails(err)

			log.Error().
				Err(err).
				Str("chat_id", chatID).
				Str("error_type", fmt.Sprintf("%T", err)).
				Str("error_code", errorCode).
				Str("error_message", errorMessage).
				Str("error_details", errorDetails).
				Msg("Failed to send message to Teams chat")

			// Check for specific error codes and messages
			fullError := strings.ToLower(errorCode + " " + errorMessage)

			if strings.Contains(fullError, "license") {
				return nil, fmt.Errorf("failed to send message: This operation requires a Microsoft Teams license. Error: %s - %s", errorCode, errorMessage)
			}
			if strings.Contains(fullError, "forbidden") || strings.Contains(fullError, "unauthorized") || errorCode == "Forbidden" || errorCode == "Unauthorized" {
				return nil, fmt.Errorf("insufficient permissions to send message. Make sure admin consent is granted for ChatMessage.Send permission. Error: %s - %s", errorCode, errorMessage)
			}
			if strings.Contains(fullError, "not found") || errorCode == "NotFound" {
				return nil, fmt.Errorf("chat with ID '%s' not found. Error: %s - %s", chatID, errorCode, errorMessage)
			}

			return nil, fmt.Errorf("failed to send message to Teams chat: [%s] %s", errorCode, errorMessage)
		}

		log.Info().Str("chat_id", chatID).Msg("Successfully sent message to Teams chat")
		return result, nil
	}

	// Parse channel ID to get team ID and channel ID
	// Expected format: teamId:channelId
	teamID, channelID, err := parseChannelID(params.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse channel_id: %w", err)
	}

	log.Info().
		Str("team_id", teamID).
		Str("channel_id", channelID).
		Str("message_preview", func() string {
			if len(params.Message) > 50 {
				return params.Message[:50] + "..."
			}
			return params.Message
		}()).
		Msg("Sending message to Teams channel")

	// Send the message to channel
	result, err := i.graphClient.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().Post(ctx, requestBody, nil)
	if err != nil {
		// Extract detailed error information
		errorCode, errorMessage, errorDetails := extractODataErrorDetails(err)

		log.Error().
			Err(err).
			Str("team_id", teamID).
			Str("channel_id", channelID).
			Str("error_type", fmt.Sprintf("%T", err)).
			Str("error_code", errorCode).
			Str("error_message", errorMessage).
			Str("error_details", errorDetails).
			Msg("Failed to send message to Teams channel")

		// Check for specific error codes and messages
		fullError := strings.ToLower(errorCode + " " + errorMessage)

		if strings.Contains(fullError, "license") {
			return nil, fmt.Errorf("failed to send message: This operation requires a Microsoft Teams license. Error: %s - %s", errorCode, errorMessage)
		}
		if strings.Contains(fullError, "forbidden") || strings.Contains(fullError, "unauthorized") || errorCode == "Forbidden" || errorCode == "Unauthorized" {
			return nil, fmt.Errorf("insufficient permissions to send message. Make sure admin consent is granted for ChannelMessage.Send permission. Error: %s - %s", errorCode, errorMessage)
		}
		if strings.Contains(fullError, "not found") || errorCode == "NotFound" {
			return nil, fmt.Errorf("channel or team not found. Make sure team ID '%s' and channel ID '%s' are correct. Error: %s - %s", teamID, channelID, errorCode, errorMessage)
		}

		return nil, fmt.Errorf("failed to send message to Teams channel: [%s] %s", errorCode, errorMessage)
	}

	log.Info().
		Str("team_id", teamID).
		Str("channel_id", channelID).
		Msg("Successfully sent message to Teams channel")

	return result, nil
}

// CreateChannel creates a new channel in a Teams team
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

	log.Info().
		Str("team_id", params.TeamID).
		Str("channel_name", params.ChannelName).
		Str("channel_type", func() string {
			if params.ChannelType != nil {
				return *params.ChannelType
			}
			return "standard"
		}()).
		Msg("Creating Teams channel")

	// Determine channel membership type
	membershipType := models.STANDARD_CHANNELMEMBERSHIPTYPE
	if params.ChannelType != nil && *params.ChannelType == "private" {
		membershipType = models.PRIVATE_CHANNELMEMBERSHIPTYPE
	}

	// Create the channel
	requestBody := models.NewChannel()
	requestBody.SetDisplayName(&params.ChannelName)
	requestBody.SetMembershipType(&membershipType)

	if params.ChannelDescription != nil && *params.ChannelDescription != "" {
		requestBody.SetDescription(params.ChannelDescription)
	}

	// Create channel in the team
	result, err := i.graphClient.Teams().ByTeamId(params.TeamID).Channels().Post(ctx, requestBody, nil)
	if err != nil {
		// Extract detailed error information
		errorCode, errorMessage, errorDetails := extractODataErrorDetails(err)

		log.Error().
			Err(err).
			Str("team_id", params.TeamID).
			Str("channel_name", params.ChannelName).
			Str("error_type", fmt.Sprintf("%T", err)).
			Str("error_code", errorCode).
			Str("error_message", errorMessage).
			Str("error_details", errorDetails).
			Msg("Failed to create Teams channel")

		// Check for specific error codes and messages
		fullError := strings.ToLower(errorCode + " " + errorMessage)

		if strings.Contains(fullError, "license") {
			return nil, fmt.Errorf("failed to create channel: This operation requires a Microsoft Teams license. Error: %s - %s", errorCode, errorMessage)
		}
		if strings.Contains(fullError, "duplicate") || strings.Contains(fullError, "already exists") || strings.Contains(fullError, "conflict") {
			return nil, fmt.Errorf("channel '%s' already exists in this team. Error: %s - %s", params.ChannelName, errorCode, errorMessage)
		}
		if strings.Contains(fullError, "forbidden") || strings.Contains(fullError, "unauthorized") || errorCode == "Forbidden" || errorCode == "Authorization_RequestDenied" {
			return nil, fmt.Errorf("insufficient permissions to create channel. Make sure admin consent is granted for Channel.Create permission. Error: %s - %s", errorCode, errorMessage)
		}
		if strings.Contains(fullError, "not found") || errorCode == "NotFound" {
			return nil, fmt.Errorf("team with ID '%s' not found. Make sure the team exists and you have access. Error: %s - %s", params.TeamID, errorCode, errorMessage)
		}

		return nil, fmt.Errorf("failed to create Teams channel: [%s] %s", errorCode, errorMessage)
	}

	log.Info().
		Str("team_id", params.TeamID).
		Str("channel_name", params.ChannelName).
		Str("channel_id", func() string {
			if result.GetId() != nil {
				return *result.GetId()
			}
			return "unknown"
		}()).
		Msg("Successfully created Teams channel")

	return result, nil
}

// CreateTeam creates a new Microsoft Teams team
func (i *TeamsIntegration) CreateTeam(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreateTeamParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.TeamName == "" {
		return nil, fmt.Errorf("team_name is required")
	}

	log.Info().
		Str("team_name", params.TeamName).
		Str("visibility", func() string {
			if params.TeamVisibility != nil {
				return *params.TeamVisibility
			}
			return "private"
		}()).
		Msg("Creating Microsoft Teams team")

	// Determine team visibility
	visibility := "Private"
	if params.TeamVisibility != nil && *params.TeamVisibility == "public" {
		visibility = "Public"
	}

	// First, create a Microsoft 365 Group
	groupRequestBody := models.NewGroup()
	groupRequestBody.SetDisplayName(&params.TeamName)
	groupRequestBody.SetMailEnabled(func() *bool { b := true; return &b }())
	groupRequestBody.SetMailNickname(func() *string {
		// Create a valid mail nickname from team name (lowercase, no spaces)
		nickname := strings.ToLower(strings.ReplaceAll(params.TeamName, " ", "-"))
		return &nickname
	}())
	groupRequestBody.SetSecurityEnabled(func() *bool { b := false; return &b }())
	groupRequestBody.SetVisibility(&visibility)

	if params.TeamDescription != nil && *params.TeamDescription != "" {
		groupRequestBody.SetDescription(params.TeamDescription)
	}

	// Set group types to indicate it will be a Team
	groupTypes := []string{"Unified"}
	groupRequestBody.SetGroupTypes(groupTypes)

	// Create the group
	group, err := i.graphClient.Groups().Post(ctx, groupRequestBody, nil)
	if err != nil {
		// Extract detailed error information
		errorCode, errorMessage, errorDetails := extractODataErrorDetails(err)

		log.Error().
			Err(err).
			Str("team_name", params.TeamName).
			Str("error_type", fmt.Sprintf("%T", err)).
			Str("error_code", errorCode).
			Str("error_message", errorMessage).
			Str("error_details", errorDetails).
			Msg("Failed to create Microsoft 365 Group")

		// Check for specific error codes and messages
		fullError := strings.ToLower(errorCode + " " + errorMessage)

		if strings.Contains(fullError, "license") {
			return nil, fmt.Errorf("failed to create Team: This operation requires a Microsoft Teams license. Error: %s - %s", errorCode, errorMessage)
		}
		if strings.Contains(fullError, "insufficient privileges") || strings.Contains(fullError, "authorization") || errorCode == "Authorization_RequestDenied" {
			return nil, fmt.Errorf("insufficient permissions to create team. Make sure admin consent is granted for Group.ReadWrite.All permission. Error: %s - %s", errorCode, errorMessage)
		}
		if strings.Contains(fullError, "duplicate") || strings.Contains(fullError, "already exists") {
			return nil, fmt.Errorf("team or group with name '%s' already exists. Error: %s - %s", params.TeamName, errorCode, errorMessage)
		}

		return nil, fmt.Errorf("failed to create Microsoft 365 Group: [%s] %s", errorCode, errorMessage)
	}

	if group.GetId() == nil {
		return nil, fmt.Errorf("created group has no ID")
	}

	groupID := *group.GetId()

	log.Info().Str("group_id", groupID).Str("team_name", params.TeamName).Msg("Successfully created Microsoft 365 Group, now converting to Team")

	// Convert the group to a Team by adding Team settings
	teamRequestBody := models.NewTeam()

	// Set messaging settings
	messagingSettings := models.NewTeamMessagingSettings()
	allowUserEditMessages := true
	allowUserDeleteMessages := true
	messagingSettings.SetAllowUserEditMessages(&allowUserEditMessages)
	messagingSettings.SetAllowUserDeleteMessages(&allowUserDeleteMessages)
	teamRequestBody.SetMessagingSettings(messagingSettings)

	// Set fun settings
	funSettings := models.NewTeamFunSettings()
	allowGiphy := true
	funSettings.SetAllowGiphy(&allowGiphy)
	teamRequestBody.SetFunSettings(funSettings)

	// Convert group to team
	team, err := i.graphClient.Groups().ByGroupId(groupID).Team().Put(ctx, teamRequestBody, nil)
	if err != nil {
		// Extract detailed error information
		errorCode, errorMessage, errorDetails := extractODataErrorDetails(err)

		log.Error().
			Err(err).
			Str("group_id", groupID).
			Str("team_name", params.TeamName).
			Str("error_type", fmt.Sprintf("%T", err)).
			Str("error_code", errorCode).
			Str("error_message", errorMessage).
			Str("error_details", errorDetails).
			Msg("Failed to convert group to Team")

		// Check for specific error codes and messages
		fullError := strings.ToLower(errorCode + " " + errorMessage)

		if strings.Contains(fullError, "license") {
			return nil, fmt.Errorf("failed to create Team: This operation requires a Microsoft Teams license. Error: %s - %s", errorCode, errorMessage)
		}
		if strings.Contains(fullError, "insufficient privileges") || strings.Contains(fullError, "authorization") || errorCode == "Authorization_RequestDenied" {
			return nil, fmt.Errorf("insufficient permissions to convert group to team. Make sure admin consent is granted for Group.ReadWrite.All and TeamSettings.ReadWrite.All permissions. Error: %s - %s", errorCode, errorMessage)
		}

		return nil, fmt.Errorf("failed to convert group to Team: [%s] %s", errorCode, errorMessage)
	}

	log.Info().
		Str("group_id", groupID).
		Str("team_name", params.TeamName).
		Str("team_id", func() string {
			if team.GetId() != nil {
				return *team.GetId()
			}
			return groupID
		}()).
		Msg("Successfully created Teams team")

	return team, nil
}

// PeekChannels returns available Teams channels
func (i *TeamsIntegration) PeekChannels(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	var results []domain.PeekResultItem

	// Get all teams for the user (requires Teams license)
	teamsResult, err := i.graphClient.Me().JoinedTeams().Get(ctx, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get joined teams")
		return domain.PeekResult{}, fmt.Errorf("failed to get joined teams: %w. Make sure the user has a Teams license and necessary permissions are granted", err)
	}

	if teamsResult == nil || teamsResult.GetValue() == nil {
		log.Info().Msg("No teams found for user")
		return domain.PeekResult{Result: results}, nil
	}

	teamCount := len(teamsResult.GetValue())
	log.Info().Int("team_count", teamCount).Msg("Found teams via JoinedTeams API")

	for _, team := range teamsResult.GetValue() {
		if team.GetId() == nil || team.GetDisplayName() == nil {
			continue
		}

		teamID := *team.GetId()
		teamName := *team.GetDisplayName()

		log.Info().Str("team_id", teamID).Str("team_name", teamName).Msg("Getting channels for team")

		// Get channels for this team
		channelsResult, err := i.graphClient.Teams().ByTeamId(teamID).Channels().Get(ctx, nil)
		if err != nil {
			log.Warn().Err(err).Str("team_id", teamID).Str("team_name", teamName).Msg("Failed to get channels for team")
			continue
		}

		if channelsResult != nil && channelsResult.GetValue() != nil {
			channelCount := len(channelsResult.GetValue())
			log.Info().Str("team_name", teamName).Int("channel_count", channelCount).Msg("Found channels in team")

			for _, channel := range channelsResult.GetValue() {
				if channel.GetId() == nil || channel.GetDisplayName() == nil {
					continue
				}

				channelID := *channel.GetId()
				channelName := *channel.GetDisplayName()
				channelKey := fmt.Sprintf("%s:%s", teamID, channelID)

				log.Info().Str("channel_name", channelName).Str("channel_id", channelID).Str("team_name", teamName).Msg("Adding channel to results")

				results = append(results, domain.PeekResultItem{
					Key:     channelKey,
					Value:   channelKey,
					Content: fmt.Sprintf("%s / %s", teamName, channelName),
				})
			}
		}
	}

	log.Info().Int("total_channels", len(results)).Msg("Total channels found")
	return domain.PeekResult{Result: results}, nil
}

// PeekTeams returns available Teams
func (i *TeamsIntegration) PeekTeams(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	var results []domain.PeekResultItem

	// Use JoinedTeams API (requires Teams license)
	teamsResult, err := i.graphClient.Me().JoinedTeams().Get(ctx, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get joined teams")
		return domain.PeekResult{}, fmt.Errorf("failed to get joined teams: %w. Make sure the user has a Teams license and necessary permissions are granted", err)
	}

	if teamsResult == nil || teamsResult.GetValue() == nil {
		log.Info().Msg("No teams found for user")
		return domain.PeekResult{Result: results}, nil
	}

	teamCount := len(teamsResult.GetValue())
	log.Info().Int("team_count", teamCount).Msg("Found teams via JoinedTeams API")

	for _, team := range teamsResult.GetValue() {
		if team.GetId() == nil || team.GetDisplayName() == nil {
			continue
		}

		teamID := *team.GetId()
		teamName := *team.GetDisplayName()

		log.Info().Str("team_id", teamID).Str("team_name", teamName).Msg("Found Team")

		results = append(results, domain.PeekResultItem{
			Key:     teamID,
			Value:   teamID,
			Content: teamName,
		})
	}

	log.Info().Int("total_teams", len(results)).Msg("Total teams found")
	return domain.PeekResult{Result: results}, nil
}

// Helper function to extract detailed error information from ODataError
func extractODataErrorDetails(err error) (code string, message string, details string) {
	if odataErr, ok := err.(*odataerrors.ODataError); ok {
		mainErr := odataErr.GetErrorEscaped()
		if mainErr != nil {
			if mainErr.GetCode() != nil {
				code = *mainErr.GetCode()
			}
			if mainErr.GetMessage() != nil {
				message = *mainErr.GetMessage()
			}

			// Try to get inner error details
			if innerErr := mainErr.GetInnerError(); innerErr != nil {
				details = fmt.Sprintf("InnerError: %+v", innerErr)
			}
		}
	}

	if code == "" {
		code = "UnknownError"
	}
	if message == "" {
		message = err.Error()
	}

	return code, message, details
}

// Helper function to parse channel ID
func parseChannelID(channelID string) (teamID string, channelIDParsed string, err error) {
	// Split by ':'
	parts := strings.Split(channelID, ":")

	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	// If no colon, assume it's just the channel ID and we need to find the team
	return "", "", fmt.Errorf("channel_id must be in format 'teamId:channelId'")
}

// TeamsTokenCredential implements the Azure Identity credential interface
type TeamsTokenCredential struct {
	accessToken string
}

func (c *TeamsTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     c.accessToken,
		ExpiresOn: time.Now().Add(1 * time.Hour),
	}, nil
}

// Action Parameters
type SendMessageParams struct {
	ChannelID   string  `json:"channel_id"`
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

package teams

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	// Action Types
	TeamsActionType_SendMessage   domain.IntegrationActionType = "teams_send_message"
	TeamsActionType_CreateChannel domain.IntegrationActionType = "teams_create_channel"
	TeamsActionType_CreateTeam    domain.IntegrationActionType = "teams_create_team"

	// Peekable Types
	TeamsPeekable_Channels domain.IntegrationPeekableType = "teams_channels"
	TeamsPeekable_Teams    domain.IntegrationPeekableType = "teams_teams"
)

var (
	// Teams Schema defines the Microsoft Teams integration
	TeamsSchema = domain.Integration{
		ID:                domain.IntegrationType_Teams,
		Name:              "Microsoft Teams",
		Description:       "Send messages and interact with Microsoft Teams channels and chats.",
		CanTestConnection: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "teams_oauth",
				Name:        "Microsoft Teams Account",
				Description: "The Microsoft Teams account to use for the integration",
				Required:    false,
				Type:        domain.NodePropertyType_OAuth,
				OAuthType:   domain.OAuthTypeMicrosoftTeams,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "send_message",
				Name:        "Send Message",
				Description: "Send a message to a Microsoft Teams channel",
				ActionType:  TeamsActionType_SendMessage,
				Properties: []domain.NodeProperty{
					{
						Key:          "channel_id",
						Name:         "Channel or Chat ID",
						Description:  "The ID of the channel (teamId:channelId) or chat (chat:chatId). Manual entry supported.",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Channels,
						Placeholder:  "teamId:channelId or chat:chatId",
					},
					{
						Key:         "message",
						Name:        "Message",
						Description: "The message content to send",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "content_type",
						Name:        "Content Type",
						Description: "The format of the message content. Default: text",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Plain Text", Value: "text"},
							{Label: "HTML", Value: "html"},
						},
					},
				},
			},
			{
				ID:          "create_channel",
				Name:        "Create Channel",
				Description: "Create a new channel in a Microsoft Teams team",
				ActionType:  TeamsActionType_CreateChannel,
				Properties: []domain.NodeProperty{
					{
						Key:          "team_id",
						Name:         "Team",
						Description:  "The team where the channel will be created",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Teams,
					},
					{
						Key:         "channel_name",
						Name:        "Channel Name",
						Description: "The name of the channel to create",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Placeholder: "My New Channel",
					},
					{
						Key:         "channel_description",
						Name:        "Description",
						Description: "A description for the channel (optional)",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "channel_type",
						Name:        "Channel Type",
						Description: "The type of channel. Default: standard",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Standard", Value: "standard"},
							{Label: "Private", Value: "private"},
						},
					},
				},
			},
			{
				ID:          "create_team",
				Name:        "Create Team",
				Description: "Create a new Microsoft Teams team",
				ActionType:  TeamsActionType_CreateTeam,
				Properties: []domain.NodeProperty{
					{
						Key:         "team_name",
						Name:        "Team Name",
						Description: "The name of the team to create",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Placeholder: "My New Team",
					},
					{
						Key:         "team_description",
						Name:        "Description",
						Description: "A description for the team (optional)",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "team_visibility",
						Name:        "Visibility",
						Description: "The visibility of the team. Default: private",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Private", Value: "private"},
							{Label: "Public", Value: "public"},
						},
					},
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{},
	}
)

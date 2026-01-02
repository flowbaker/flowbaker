package teams

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	// Action Types
	TeamsActionType_SendChannelMessage  domain.IntegrationActionType = "teams_send_channel_message"
	TeamsActionType_SendChatMessage     domain.IntegrationActionType = "teams_send_chat_message"
	TeamsActionType_CreateChannel       domain.IntegrationActionType = "teams_create_channel"
	TeamsActionType_CreateTeam          domain.IntegrationActionType = "teams_create_team"
	TeamsActionType_DeleteChannel       domain.IntegrationActionType = "teams_delete_channel"
	TeamsActionType_GetChannel          domain.IntegrationActionType = "teams_get_channel"
	TeamsActionType_GetManyChannels     domain.IntegrationActionType = "teams_get_many_channels"
	TeamsActionType_UpdateChannel       domain.IntegrationActionType = "teams_update_channel"
	TeamsActionType_GetChannelMessages  domain.IntegrationActionType = "teams_get_channel_messages"
	TeamsActionType_GetChatMessage      domain.IntegrationActionType = "teams_get_chat_message"
	TeamsActionType_GetManyChatMessages domain.IntegrationActionType = "teams_get_many_chat_messages"

	// Trigger Event Types
	IntegrationEventType_TeamsChannelMessage domain.IntegrationTriggerEventType = "channel_message"
	IntegrationEventType_TeamsChatMessage    domain.IntegrationTriggerEventType = "chat_message"
	IntegrationEventType_TeamsMemberAdded    domain.IntegrationTriggerEventType = "member_added"
	IntegrationEventType_TeamsMemberRemoved  domain.IntegrationTriggerEventType = "member_removed"
	IntegrationEventType_TeamsChannelCreated domain.IntegrationTriggerEventType = "channel_created"
	IntegrationEventType_TeamsChannelDeleted domain.IntegrationTriggerEventType = "channel_deleted"

	// Universal trigger type for Teams
	IntegrationEventType_TeamsUniversalTrigger domain.IntegrationTriggerEventType = "teams_universal_trigger"

	// Peekable Types
	TeamsPeekable_Channels domain.IntegrationPeekableType = "teams_channels"
	TeamsPeekable_Chats    domain.IntegrationPeekableType = "teams_chats"
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
				ID:          "send_channel_message",
				Name:        "Send Channel Message",
				Description: "Send a message to a Microsoft Teams channel",
				ActionType:  TeamsActionType_SendChannelMessage,
				Properties: []domain.NodeProperty{
					{
						Key:          "channel_id",
						Name:         "Channel",
						Description:  "The channel to send the message to (teamId:channelId format)",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Channels,
						Placeholder:  "teamId:channelId",
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
						Description: "The format of the message content. Default: html",
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
				ID:          "send_chat_message",
				Name:        "Send Chat Message",
				Description: "Send a message to a Microsoft Teams chat",
				ActionType:  TeamsActionType_SendChatMessage,
				Properties: []domain.NodeProperty{
					{
						Key:          "chat_id",
						Name:         "Chat",
						Description:  "The chat to send the message to",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Chats,
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
						Description: "The format of the message content. Default: html",
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
			{
				ID:          "delete_channel",
				Name:        "Delete Channel",
				Description: "Delete a channel from a Microsoft Teams team",
				ActionType:  TeamsActionType_DeleteChannel,
				Properties: []domain.NodeProperty{
					{
						Key:          "channel_id",
						Name:         "Channel",
						Description:  "The channel to delete",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Channels,
					},
				},
			},
			{
				ID:          "get_channel",
				Name:        "Get Channel",
				Description: "Get details of a specific Microsoft Teams channel",
				ActionType:  TeamsActionType_GetChannel,
				Properties: []domain.NodeProperty{
					{
						Key:          "team_id",
						Name:         "Team",
						Description:  "The team containing the channel",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Teams,
					},
					{
						Key:         "channel_id",
						Name:        "Channel ID",
						Description: "The ID of the channel to get",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_many_channels",
				Name:        "Get Many Channels",
				Description: "Get all channels in a Microsoft Teams team",
				ActionType:  TeamsActionType_GetManyChannels,
				Properties: []domain.NodeProperty{
					{
						Key:          "team_id",
						Name:         "Team",
						Description:  "The team to get channels from",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Teams,
					},
				},
			},
			{
				ID:          "update_channel",
				Name:        "Update Channel",
				Description: "Update a Microsoft Teams channel's name or description",
				ActionType:  TeamsActionType_UpdateChannel,
				Properties: []domain.NodeProperty{
					{
						Key:          "team_id",
						Name:         "Team",
						Description:  "The team containing the channel",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Teams,
					},
					{
						Key:         "channel_id",
						Name:        "Channel ID",
						Description: "The ID of the channel to update",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "channel_name",
						Name:        "New Channel Name",
						Description: "The new name for the channel (optional)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "channel_description",
						Name:        "New Description",
						Description: "The new description for the channel (optional)",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "get_channel_messages",
				Name:        "Get Channel Messages",
				Description: "Get messages from a Microsoft Teams channel",
				ActionType:  TeamsActionType_GetChannelMessages,
				Properties: []domain.NodeProperty{
					{
						Key:          "channel_id",
						Name:         "Channel",
						Description:  "The channel to get messages from",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Channels,
					},
					{
						Key:         "top",
						Name:        "Number of Messages",
						Description: "Maximum number of messages to retrieve (optional)",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
						Placeholder: "50",
					},
				},
			},
			{
				ID:          "get_chat_message",
				Name:        "Get Chat Message",
				Description: "Get a specific message from a Microsoft Teams chat",
				ActionType:  TeamsActionType_GetChatMessage,
				Properties: []domain.NodeProperty{
					{
						Key:          "chat_id",
						Name:         "Chat",
						Description:  "The chat to get message from",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Chats,
					},
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The ID of the message to get",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_many_chat_messages",
				Name:        "Get Many Chat Messages",
				Description: "Get messages from a Microsoft Teams chat",
				ActionType:  TeamsActionType_GetManyChatMessages,
				Properties: []domain.NodeProperty{
					{
						Key:          "chat_id",
						Name:         "Chat",
						Description:  "The chat to get messages from",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Chats,
					},
					{
						Key:         "top",
						Name:        "Number of Messages",
						Description: "Maximum number of messages to retrieve (optional)",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
						Placeholder: "50",
					},
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "teams_event_listener",
				Name:        "Microsoft Teams Event Listener",
				Description: "Triggers on selected Microsoft Teams events for channels, chats, and teams.",
				EventType:   IntegrationEventType_TeamsUniversalTrigger,
				Properties: []domain.NodeProperty{
					{
						Key:          "channel_id",
						Name:         "Channel",
						Description:  "The channel to monitor for events (format: teamId:channelId)",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: TeamsPeekable_Channels,
						Placeholder:  "teamId:channelId",
					},
					{
						Key:         "selected_events",
						Name:        "Teams Events",
						Description: "Select one or more Microsoft Teams events to trigger this flow.",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 0,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:      "event",
									Name:     "Event",
									Type:     domain.NodePropertyType_String,
									Required: true,
									Options: []domain.NodePropertyOption{
										{Label: "On Channel Message Posted", Value: string(IntegrationEventType_TeamsChannelMessage), Description: "Triggered when a message is posted in a channel (works with Business Basic)"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
)

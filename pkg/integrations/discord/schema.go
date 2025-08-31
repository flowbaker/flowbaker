package discord

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                domain.IntegrationType_Discord,
		Name:              "Discord",
		Description:       "Use Discord integration to send messages to channels, create channels, and more.",
		CanTestConnection: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "token",
				Name:        "Token",
				Description: "The token of the bot. You can get this by going to the Discord developer portal",
				Required:    true,
				Type:        domain.NodePropertyType_String,
				IsSecret:    true,
			},
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "on_message_received",
				Name:        "On Message Received",
				EventType:   IntegrationTriggerType_MessageReceived,
				Description: "Triggered when a message is received in a channel",
				Properties: []domain.NodeProperty{
					{
						Key:              "guild_id",
						Name:             "Guild",
						Description:      "The ID of the guild to send the message to",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DiscordIntegrationPeekable_Guilds,
						ExpressionChoice: true,
					},
					{
						Key:          "channel_id",
						Name:         "Channel ID",
						Description:  "The ID of the channel to send the message to",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: DiscordIntegrationPeekable_Channels,
						Dependent:    []string{"guild_id"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "guild_id",
								ValueKey:    "guild_id",
							},
						},
						ExpressionChoice: true,
					},
				},
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "send_message",
				Name:        "Send Message",
				ActionType:  IntegrationActionType_SendMessage,
				Description: "Send a message to a channel",
				Properties: []domain.NodeProperty{
					{
						Key:              "guild_id",
						Name:             "Guild",
						Description:      "The ID of the guild to send the message to",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DiscordIntegrationPeekable_Guilds,
						ExpressionChoice: true,
					},
					{
						Key:          "channel_id",
						Name:         "Channel ID",
						Description:  "The ID of the channel to send the message to",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: DiscordIntegrationPeekable_Channels,
						Dependent:    []string{"guild_id"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "guild_id",
								ValueKey:    "guild_id",
							},
						},
						ExpressionChoice: true,
					},
					{
						Key:         "content",
						Name:        "Content",
						Description: "The text content of the message",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "get_messages",
				Name:        "Get Messages",
				ActionType:  IntegrationActionType_GetMessages,
				Description: "Get messages from channel",
				Properties: []domain.NodeProperty{
					{
						Key:              "guild_id",
						Name:             "Guild",
						Description:      "The ID of the guild to get messages",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DiscordIntegrationPeekable_Guilds,
						ExpressionChoice: true,
					},
					{
						Key:          "channel_id",
						Name:         "Channel ID",
						Description:  "The ID of the channel to get messages",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: DiscordIntegrationPeekable_Channels,
						Dependent:    []string{"guild_id"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "guild_id",
								ValueKey:    "guild_id",
							},
						},
						ExpressionChoice: true,
					},
					{
						Key:         "amount",
						Name:        "Amount",
						Description: "The amount of messages",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "before_id",
						Name:        "Before ID",
						Description: "Get messages before specified message",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "after_id",
						Name:        "After ID",
						Description: "Get messages after specified message",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_message_by_id",
				Name:        "Get Message",
				ActionType:  IntegrationActionType_GetMessageByID,
				Description: "Get message from channel by id",
				Properties: []domain.NodeProperty{
					{
						Key:              "guild_id",
						Name:             "Guild",
						Description:      "The ID of the guild to get the message to",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DiscordIntegrationPeekable_Guilds,
						ExpressionChoice: true,
					},
					{
						Key:          "channel_id",
						Name:         "Channel ID",
						Description:  "The ID of the channel to get the message to",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: DiscordIntegrationPeekable_Channels,
						Dependent:    []string{"guild_id"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "guild_id",
								ValueKey:    "guild_id",
							},
						},
						ExpressionChoice: true,
					},
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "Message ID to get message",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "delete_message_by_id",
				Name:        "Delete Message",
				ActionType:  IntegrationActionType_DeleteMessage,
				Description: "Delete message from channel by id",
				Properties: []domain.NodeProperty{
					{
						Key:              "guild_id",
						Name:             "Guild",
						Description:      "The ID of the guild to delete the message to",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     DiscordIntegrationPeekable_Guilds,
						ExpressionChoice: true,
					},
					{
						Key:          "channel_id",
						Name:         "Channel ID",
						Description:  "The ID of the channel to delete the message to",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: DiscordIntegrationPeekable_Channels,
						Dependent:    []string{"guild_id"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "guild_id",
								ValueKey:    "guild_id",
							},
						},
						ExpressionChoice: true,
					},
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "Message ID to delete message",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
		},
	}
)

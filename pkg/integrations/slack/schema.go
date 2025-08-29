package slackintegration

import (
	"flowbaker/internal/domain"
)

var (
	IntegrationEventType_SlackMessageReceived domain.IntegrationTriggerEventType = "slack_message_received"
	IntegrationEventType_SlackReactionAdded   domain.IntegrationTriggerEventType = "slack_reaction_added"
)

var (
	SlackSchema = domain.Integration{
		ID:          domain.IntegrationType_Slack,
		Name:        "Slack",
		Description: "Send messages to Slack channels and users.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "slack_oauth",
				Name:        "Slack Account",
				Description: "The Slack account to use for the integration",
				Required:    false,
				Type:        domain.NodePropertyType_OAuth,
				OAuthType:   domain.OAuthTypeSlack,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "send_message",
				Name:        "Send Message",
				Description: "Send a message to a Slack channel",
				ActionType:  SlackIntegrationActionType_SendMessage,
				Properties: []domain.NodeProperty{
					{
						Key:          "channel_id",
						Name:         "Channel",
						Description:  "The channel to send the message to",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: SlackIntegrationPeekable_Channels,
					},
					{
						Key:         "message",
						Name:        "Message",
						Description: "The message to send",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "get_message",
				Name:        "Get Message",
				Description: "Get a message from a Slack channel",
				ActionType:  SlackIntegrationActionType_GetMessage,
				Properties: []domain.NodeProperty{
					{
						Key:          "channel_id",
						Name:         "Channel",
						Description:  "The channel to get the message from",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: SlackIntegrationPeekable_Channels,
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
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "slack_message_received",
				Name:        "Slack Message Received",
				EventType:   IntegrationEventType_SlackMessageReceived,
				Description: "Triggered when a message is received in a Slack channel",
				Properties: []domain.NodeProperty{
					{
						Key:          "channel_id",
						Name:         "Channel",
						Description:  "The channel to listen for messages",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: SlackIntegrationPeekable_Channels,
					},
				},
			},
			{
				ID:          "slack_reaction_added",
				Name:        "Reaction Added",
				EventType:   IntegrationEventType_SlackReactionAdded,
				Description: "Triggered when a reaction is added to a message in a Slack channel",
				Properties: []domain.NodeProperty{
					{
						Key:          "channel_id",
						Name:         "Channel",
						Description:  "The channel to listen for reactions",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: SlackIntegrationPeekable_Channels,
					},
				},
			},
		},
	}
)

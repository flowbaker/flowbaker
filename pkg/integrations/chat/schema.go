package chat

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationTriggerType_ChatMessageReceived domain.IntegrationTriggerEventType = "chat_message_received"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_ChatTrigger,
		Name:        "Chat Trigger",
		Description: "Trigger workflows from chat messages",
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "chat_message_received",
				Name:        "Chat Message Received",
				EventType:   IntegrationTriggerType_ChatMessageReceived,
				Description: "Triggered when a chat message is received",
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Output: []domain.NodeHandle{
							{
								Type:         domain.NodeHandleTypeDefault,
								Position:     domain.NodeHandlePositionBottom,
								Text:         "Agent",
								UsageContext: domain.UsageContextWorkflow,
								AllowedIntegrations: []domain.IntegrationType{
									domain.IntegrationType_AIAgent,
								},
							},
						},
					},
				},
			},
		},
	}
)

package flowbaker_agent_memory

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	ActionUseMemory domain.IntegrationActionType = "flowbaker_agent_use_memory"
)

var (
	Schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_FlowbakerAgentMemory,
		Name:        "Conversation Memory",
		Description: "Access and manage agent conversation memory using Flowbaker's built-in memory system",
		Actions: []domain.IntegrationAction{
			{
				ID:          "flowbaker_agent_use_memory",
				ActionType:  ActionUseMemory,
				Name:        "Use Memory Context",
				Description: "Retrieve agent conversation history to provide context for AI interactions",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextMemoryProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:              "session_id",
						Name:             "Session ID",
						Description:      "The session ID to use for the conversation memory",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						ExpressionChoice: true,
					},
				},
			},
		},
	}
)

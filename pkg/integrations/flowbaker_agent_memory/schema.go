package flowbaker_agent_memory

import (
	"github.com/flowbaker/flowbaker/internal/domain"
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
					{
						Key:              "session_ttl",
						Name:             "Session Expiration in seconds",
						Description:      "The expiration time for the session in seconds",
						Required:         true,
						Type:             domain.NodePropertyType_Integer,
						ExpressionChoice: true,
					},
					{
						Key:              "conversation_count",
						Name:             "Conversation Count",
						Description:      "The number of conversations to retrieve from memory",
						Required:         true,
						Type:             domain.NodePropertyType_Integer,
						ExpressionChoice: true,
					},
					{
						Key:              "max_context_length",
						Name:             "Max Context Length",
						Description:      "The maximum length of the memory context to retrieve",
						Required:         true,
						Type:             domain.NodePropertyType_Integer,
						ExpressionChoice: true,
					},
				},
			},
		},
	}
)

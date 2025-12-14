package ai_agent

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

// IntegrationActionType_AIAgentV2 is defined in ai_agent_integration_v2.go

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_AIAgent,
		Name:                 "AI Agent",
		Description:          "Advanced AI Agent integration supporting both ReAct (Reasoning + Acting) and Function Calling patterns, with tool calling, memory management, and structured conversation handling",
		CredentialProperties: []domain.NodeProperty{}, // No credentials needed
		Actions: []domain.IntegrationAction{
			{
				ID:          "ai_agent",
				Name:        "Tool Agent",
				ActionType:  IntegrationActionType_FunctionCallingAgent,
				Description: "Use Tool Agent to complete tasks using available tools",
				Properties: []domain.NodeProperty{
					{
						Key:         "system_prompt",
						Name:        "System Prompt",
						Description: "The system prompt for the AI agent",
						Type:        domain.NodePropertyType_Text,
						Required:    false,
						Placeholder: "The system prompt for the AI agent",
						Help:        "The system prompt for the AI agent",
					},
					{
						Key:         "prompt",
						Name:        "Prompt",
						Description: "The task description or prompt to give the AI agent",
						Type:        domain.NodePropertyType_Text,
						Required:    true,
						Placeholder: "Describe the task you want the AI agent to complete...",
						Help:        "Be specific about what you want the agent to accomplish. The agent will use available tools to complete this task.",
						MinLength:   1,
						MaxLength:   2000,
					},
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionTop, Text: "Input", UsageContext: domain.UsageContextWorkflow},
							{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionRight, Text: "Tools", UsageContext: domain.UsageContextTool},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionBottom, Text: "Output", UsageContext: domain.UsageContextWorkflow},
							{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionLeft, Text: "Agent", UsageContext: domain.UsageContextTool},
						},
					},
				},
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
			},
		},
		Triggers:          []domain.IntegrationTrigger{},
		CanTestConnection: false,
	}
)

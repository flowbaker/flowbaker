package ai_agent

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_AIAgent,
		Name:                 "AI Agent",
		Description:          "AI Agent that can use tools to complete tasks",
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
					{
						Key:         "max_steps",
						Name:        "Max Steps",
						Description: "The maximum number of steps the AI agent can take. (Default: 10)",
						Type:        domain.NodePropertyType_Number,
						Placeholder: "30",
						Help:        "The maximum number of steps the AI agent can take. (Default: 10)",
						NumberOpts: &domain.NumberPropertyOptions{
							Min:     0,
							Default: 10,
							Step:    1,
						},
					},
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Index: 0, Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionTop, Text: "Input", UsageContext: domain.UsageContextWorkflow},
							{Index: 3, Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionRight, Text: "Tools", UsageContext: domain.UsageContextTool},
						},
						Output: []domain.NodeHandle{
							{Index: 2, Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionBottom, Text: "Output", UsageContext: domain.UsageContextWorkflow},
						},
					},
					domain.UsageContextTool: {
						Input: []domain.NodeHandle{
							{Index: 3, Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionRight, Text: "Tools", UsageContext: domain.UsageContextTool},
						},
						Output: []domain.NodeHandle{
							{Index: 1, Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionLeft, Text: "Agent", UsageContext: domain.UsageContextTool},
						},
					},
				},
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
					domain.UsageContextTool,
				},
				CombinedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
					domain.UsageContextTool,
				},
			},
		},
		Triggers:          []domain.IntegrationTrigger{},
		CanTestConnection: false,
	}
)

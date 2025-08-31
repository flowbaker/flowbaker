package claudeintegration

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	ClaudeSchema = domain.Integration{
		ID:          domain.IntegrationType_Anthropic,
		Name:        "Antropic (Claude)",
		Description: "Generate content and analyze text using Anthropic's Claude AI models.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "api_key",
				Name:        "API Key",
				Description: "The Anthropic API key for authentication",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "prompt",
				Name:        "Send Prompt",
				Description: "Send a prompt to Claude AI and get a response",
				ActionType:  ClaudeIntegrationActionType_Prompt,
				Properties: []domain.NodeProperty{
					{
						Key:          "model",
						Name:         "Model",
						Description:  "The Claude model to use for generation",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: ClaudeIntegrationPeekable_Models,
					},
					{
						Key:         "prompt",
						Name:        "Prompt",
						Description: "The main prompt to send to Claude",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "system_prompt",
						Name:        "System Prompt",
						Description: "Optional system prompt to set Claude's behavior",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "max_tokens",
						Name:        "Max Tokens",
						Description: "Maximum number of tokens to generate. If not specified, defaults to 1024.",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "ai_agent_chat",
				Name:        "AI Agent Chat",
				ActionType:  "ai_agent_chat",
				Description: "Use Claude for AI agent conversation",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextLLMProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:          "model",
						Name:         "Model",
						Description:  "The Claude model to use",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: ClaudeIntegrationPeekable_Models,
					},
					{
						Key:         "system_prompt",
						Name:        "System Prompt",
						Description: "The system prompt for the AI agent",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{},
	}
)

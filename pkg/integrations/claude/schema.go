package claudeintegration

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	ClaudeSchema = domain.Integration{
		ID:          domain.IntegrationType_Anthropic,
		Name:        "Anthropic",
		Description: "Use Anthropic's Claude AI models to generate content and analyze text.",
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
						Key:         "model",
						Name:        "Model",
						Description: "The Claude model to use for generation",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options:     modelOptions,
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
						Key:         "model",
						Name:        "Model",
						Description: "The Claude model to use",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options:     modelOptions,
					},
					{
						Key:         "temperature",
						Name:        "Temperature",
						Description: "Controls randomness in the output. Higher values make output more random",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
						Advanced:    true,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:  0,
							Max:  1,
							Step: 0.1,
						},
					},
					{
						Key:         "max_tokens",
						Name:        "Max Tokens",
						Description: "The maximum number of tokens to generate",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "top_p",
						Name:        "Top P",
						Description: "The top P value to use for nucleus sampling",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
						Advanced:    true,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:  0,
							Max:  1,
							Step: 0.01,
						},
					},
					{
						Key:         "top_k",
						Name:        "Top K",
						Description: "Only sample from the top K options for each subsequent token",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
						Advanced:    true,
					},
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{},
	}
)

var modelOptions = []domain.NodePropertyOption{
	// Claude 4.5 models (Latest)
	{Label: "Claude Opus 4.5", Value: "claude-opus-4-5-20251101"},
	{Label: "Claude Sonnet 4.5", Value: "claude-sonnet-4-5-20250929"},
	{Label: "Claude Haiku 4.5", Value: "claude-haiku-4-5-20251001"},

	// Claude 4.1 models
	{Label: "Claude Opus 4.1", Value: "claude-opus-4-1-20250805"},

	// Claude 4 models
	{Label: "Claude Opus 4", Value: "claude-opus-4-20250514"},
	{Label: "Claude Sonnet 4", Value: "claude-sonnet-4-20250514"},

	// Claude 3.7 models
	{Label: "Claude Sonnet 3.7", Value: "claude-3-7-sonnet-20250219"},

	// Claude 3.5 models
	{Label: "Claude Haiku 3.5", Value: "claude-3-5-haiku-20241022"},

	// Claude 3 models (Legacy)
	{Label: "Claude Haiku 3", Value: "claude-3-haiku-20240307"},
	{Label: "Claude Opus 3", Value: "claude-3-opus-20240229"},
}

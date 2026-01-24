package gemini

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	GeminiSchema = domain.Integration{
		ID:          domain.IntegrationType_Gemini,
		Name:        "Google Gemini",
		Description: "Use Google's Gemini AI models to generate content and analyze text.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "api_key",
				Name:        "API Key",
				Description: "The Google AI API key for authentication",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "ai_agent_chat",
				Name:        "AI Agent Chat",
				ActionType:  "ai_agent_chat",
				Description: "Use Gemini for AI agent conversation",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextLLMProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "model",
						Name:        "Model",
						Description: "The Gemini model to use",
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
							Max:  2,
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
	// Gemini 3 models (Preview)
	{Label: "Gemini 3 Pro (Preview)", Value: "gemini-3-pro-preview"},
	{Label: "Gemini 3 Flash (Preview)", Value: "gemini-3-flash-preview"},

	// Gemini 2.5 models (Stable)
	{Label: "Gemini 2.5 Pro", Value: "gemini-2.5-pro"},
	{Label: "Gemini 2.5 Flash", Value: "gemini-2.5-flash"},
	{Label: "Gemini 2.5 Flash Lite", Value: "gemini-2.5-flash-lite"},

	// Gemini 2.0 models
	{Label: "Gemini 2.0 Flash", Value: "gemini-2.0-flash"},
	{Label: "Gemini 2.0 Flash Lite", Value: "gemini-2.0-flash-lite"},
}

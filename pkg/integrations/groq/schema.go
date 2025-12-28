package groq

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	GroqIntegrationActionType_Prompt = "prompt"
)

var (
	GroqSchema = domain.Integration{
		ID:          domain.IntegrationType_Groq,
		Name:        "Groq",
		Description: "Ultra-fast LLM inference powered by Groq's LPU technology.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "api_key",
				Name:        "API Key",
				Description: "The Groq API key for authentication",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "prompt",
				Name:        "Send Prompt",
				Description: "Send a prompt to Groq and get a response",
				ActionType:  GroqIntegrationActionType_Prompt,
				Properties: []domain.NodeProperty{
					{
						Key:         "model",
						Name:        "Model",
						Description: "The Groq model to use for generation",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options:     modelOptions,
					},
					{
						Key:         "prompt",
						Name:        "Prompt",
						Description: "The main prompt to send to Groq",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "system_prompt",
						Name:        "System Prompt",
						Description: "Optional system prompt to set the model's behavior",
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
				Description: "Use Groq for AI agent conversation",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextLLMProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "model",
						Name:        "Model",
						Description: "The Groq model to use",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options:     modelOptions,
					},
					{
						Key:         "system_prompt",
						Name:        "System Prompt",
						Description: "The system prompt for the AI agent",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
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
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{},
	}
)

var modelOptions = []domain.NodePropertyOption{
	{Label: "Groq Compound", Value: "groq/compound"},
	{Label: "Groq Compound Mini", Value: "groq/compound-mini"},
	{Label: "GPT-OSS 120B", Value: "openai/gpt-oss-120b"},
	{Label: "GPT-OSS 20B", Value: "openai/gpt-oss-20b"},
	{Label: "Llama 4 Maverick 17B", Value: "meta-llama/llama-4-maverick-17b-128e-instruct"},
	{Label: "Llama 4 Scout 17B", Value: "meta-llama/llama-4-scout-17b-16e-instruct"},
	{Label: "Llama 3.3 70B Versatile", Value: "llama-3.3-70b-versatile"},
	{Label: "Llama 3.1 8B Instant", Value: "llama-3.1-8b-instant"},
	{Label: "Kimi K2 Instruct (262K context)", Value: "moonshotai/kimi-k2-instruct-0905"},
	{Label: "Kimi K2 Instruct", Value: "moonshotai/kimi-k2-instruct"},
	{Label: "Qwen 3 32B", Value: "qwen/qwen3-32b"},
}

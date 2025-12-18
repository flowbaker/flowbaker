package openai

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_OpenAI,
		Name:        "OpenAI",
		Description: "Use OpenAI's APIs for chat completion, text generation, image creation, and embeddings.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "api_key",
				Name:        "API Key",
				Description: "Your OpenAI API key. You can get this from your OpenAI dashboard",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "chat_completion",
				Name:        "Chat Completion",
				ActionType:  IntegrationActionType_Chat,
				Description: "Generate chat completions using OpenAI's GPT models",
				Properties: []domain.NodeProperty{
					{
						Key:         "model",
						Name:        "Model",
						Description: "The GPT model to use for completion",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options:     modelOptions,
					},
					{
						Key:         "messages",
						Name:        "Messages",
						Description: "Array of messages comprising the conversation",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:      "role",
									Name:     "Role",
									Required: true,
									Type:     domain.NodePropertyType_String,
									Options: []domain.NodePropertyOption{
										{Label: "System", Value: "system"},
										{Label: "User", Value: "user"},
										{Label: "Assistant", Value: "assistant"},
									},
								},
								{
									Key:         "content",
									Name:        "Content",
									Description: "The content of the message",
									Required:    true,
									Type:        domain.NodePropertyType_Array,
									MinLength:   1,
									MaxLength:   4000,
									Placeholder: "Enter your message here...",
									ArrayOpts: &domain.ArrayPropertyOptions{
										MinItems: 1,
										MaxItems: 100,
										ItemType: domain.NodePropertyType_Map,
										ItemProperties: []domain.NodeProperty{
											{
												Key:         "type",
												Name:        "Type",
												Description: "The type of content",
												Required:    true,
												Type:        domain.NodePropertyType_String,
												Options: []domain.NodePropertyOption{
													{Label: "Text", Value: "text"},
													{Label: "Image", Value: "image"},
												},
											},
											{
												Key:         "text",
												Name:        "Text",
												Description: "The text content",
												Required:    false,
												Type:        domain.NodePropertyType_Text,
												MinLength:   1,
												MaxLength:   4000,
												Placeholder: "Enter your message here...",
												DependsOn: &domain.DependsOn{
													PropertyKey: "type",
													Value:       "text",
												},
											},
											{
												Key:         "image",
												Name:        "Image",
												Description: "The image content",
												Required:    false,
												Type:        domain.NodePropertyType_String,
												Placeholder: "https://example.com/image.jpg",
												DependsOn: &domain.DependsOn{
													PropertyKey: "type",
													Value:       "image",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Key:         "temperature",
						Name:        "Temperature",
						Description: "Controls randomness in the output. Higher values make output more random",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
						Advanced:    true,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:     0,
							Max:     2,
							Default: 0.7,
							Step:    0.1,
						},
					},
					{
						Key:         "max_tokens",
						Name:        "Max Tokens",
						Description: "The maximum number of tokens to generate",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
						Advanced:    true,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:     1,
							Max:     4096,
							Default: 150,
							Step:    1,
						},
					},
				},
			},
			{
				ID:          "image_generation",
				Name:        "Generate Image",
				ActionType:  IntegrationActionType_GenerateImage,
				Description: "Generate images using DALL-E models",
				Properties: []domain.NodeProperty{
					{
						Key:         "prompt",
						Name:        "Prompt",
						Description: "The image generation prompt",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
						MinLength:   1,
						MaxLength:   4000,
						Placeholder: "A detailed description of the image you want to generate...",
						Help:        "Be specific and detailed in your description. Include information about style, mood, lighting, and composition.",
					},
					{
						Key:         "model",
						Name:        "Model",
						Description: "The DALL-E model to use",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "DALL-E 3", Value: "dall-e-3"},
							{Label: "DALL-E 2", Value: "dall-e-2"},
						},
					},
					{
						Key:         "size",
						Name:        "Size",
						Description: "The size of the generated image",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "1024x1024", Value: "1024x1024"},
							{Label: "1024x1792", Value: "1024x1792"},
							{Label: "1792x1024", Value: "1792x1024"},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "model",
							Value:       "dall-e-3",
						},
					},
					{
						Key:         "quality",
						Name:        "Quality",
						Description: "The quality of the generated image",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Standard", Value: "standard"},
							{Label: "HD", Value: "hd"},
						},
						Advanced: true,
						DependsOn: &domain.DependsOn{
							PropertyKey: "model",
							Value:       "dall-e-3",
						},
					},
					{
						Key:         "style",
						Name:        "Style",
						Description: "The style of the generated image",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						DependsOn: &domain.DependsOn{
							PropertyKey: "model",
							Value:       "dall-e-3",
						},
						Options: []domain.NodePropertyOption{
							{Label: "Vivid", Value: "vivid"},
							{Label: "Natural", Value: "natural"},
						},
					},
					{
						Key:         "count",
						Name:        "Count",
						Description: "The number of images to generate. Only available for DALL-E 2",
						Required:    false,
						HideIf: &domain.HideIf{
							PropertyKey: "model",
							Values:      []any{"dall-e-3"},
						},
						Type: domain.NodePropertyType_Integer,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:     1,
							Max:     10,
							Default: 1,
							Step:    1,
						},
					},
				},
			},
			{
				ID:          "ai_agent_chat",
				Name:        "AI Agent Chat",
				ActionType:  "ai_agent_chat",
				Description: "Use OpenAI for AI agent conversation",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextLLMProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "model",
						Name:        "Model",
						Description: "The GPT model to use",
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
						NumberOpts: &domain.NumberPropertyOptions{
							Min: 0,
							Max: 1,
						},
					},
					{
						Key:         "frequency_penalty",
						Name:        "Frequency Penalty",
						Description: "The frequency penalty to use for generation",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:  0,
							Max:  1,
							Step: 0.01,
						},
					},
					{
						Key:         "presence_penalty",
						Name:        "Presence Penalty",
						Description: "The presence penalty to use for generation",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:  0,
							Max:  1,
							Step: 0.01,
						},
					},
					{
						Key:         "reasoning_effort",
						Name:        "Reasoning Effort",
						Description: "The reasoning effort to use for generation",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Low", Value: "low"},
							{Label: "Medium", Value: "medium"},
							{Label: "High", Value: "high"},
						},
					},
					{
						Key:         "verbosity",
						Name:        "Verbosity",
						Description: "The verbosity of the generation",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Low", Value: "low"},
							{Label: "Medium", Value: "medium"},
							{Label: "High", Value: "high"},
						},
					},
				},
			},
			/* 			{
				ID:          "generate_embeddings",
				Name:        "Generate Embeddings",
				ActionType:  "generate_embeddings",
				Description: "Generate text embeddings using OpenAI's embedding models",
				Properties: []domain.NodeProperty{
					{
						Key:         "model",
						Name:        "Model",
						Description: "The embedding model to use",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Text Embedding 3 Small", Value: "text-embedding-3-small"},
							{Label: "Text Embedding 3 Large", Value: "text-embedding-3-large"},
							{Label: "Text Embedding Ada 002", Value: "text-embedding-ada-002"},
						},
					},
					{
						Key:         "input",
						Name:        "Input",
						Description: "Text input or array of texts to generate embeddings for",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 2048,
							ItemType: domain.NodePropertyType_String,
						},
					},
				},
			}, */
		},
		EmbeddingModels: []domain.IntegrationEmbeddingModel{
			{
				ID:          "internal-text-embedding-3-large",
				IsInternal:  true,
				Name:        "Text Embedding 3 Large",
				Description: "Text embedding model for text search",
			},
			{
				ID:          "text-embedding-3-small",
				Name:        "Text Embedding 3 Small",
				Description: "Text embedding model for text search",
			},
			{
				ID:          "text-embedding-3-large",
				Name:        "Text Embedding 3 Large",
				Description: "Text embedding model for text search",
			},
		},
	}
)

var modelOptions = []domain.NodePropertyOption{
	// GPT-5.2 series (Dec 2025 - Latest)
	{Label: "GPT-5.2", Value: "gpt-5.2"},

	// GPT-5.1 series (Nov 2025)
	{Label: "GPT-5.1", Value: "gpt-5.1"},

	// GPT-5 series (Aug 2025)
	{Label: "GPT-5", Value: "gpt-5"},
	{Label: "GPT-5 Mini", Value: "gpt-5-mini"},
	{Label: "GPT-5 Nano", Value: "gpt-5-nano"},
	{Label: "GPT-5 Chat Latest", Value: "gpt-5-chat-latest"},

	// O-series models (2025)
	{Label: "O4 Mini", Value: "o4-mini"},
	{Label: "O3", Value: "o3"},
	{Label: "O3 Mini", Value: "o3-mini"},

	// GPT-4.1 series (Apr 2025)
	{Label: "GPT-4.1", Value: "gpt-4.1"},
	{Label: "GPT-4.1 Mini", Value: "gpt-4.1-mini"},
	{Label: "GPT-4.1 Nano", Value: "gpt-4.1-nano"},

	// GPT-4.5 series (Feb 2025)
	{Label: "GPT-4.5", Value: "gpt-4.5"},

	// GPT-4o series (2024)
	{Label: "GPT-4o", Value: "gpt-4o"},
	{Label: "ChatGPT-4o Latest", Value: "chatgpt-4o-latest"},
	{Label: "GPT-4o Mini", Value: "gpt-4o-mini"},

	// O1 series (2024)
	{Label: "O1", Value: "o1"},

	// GPT-4 Turbo series (2024)
	{Label: "GPT-4 Turbo", Value: "gpt-4-turbo"},
	{Label: "GPT-4 Turbo Preview", Value: "gpt-4-turbo-preview"},

	// GPT-4 series
	{Label: "GPT-4", Value: "gpt-4"},

	// GPT-3.5 series (2023-2024)
	{Label: "GPT-3.5 Turbo 0125", Value: "gpt-3.5-turbo-0125"},
	{Label: "GPT-3.5 Turbo", Value: "gpt-3.5-turbo"},
}

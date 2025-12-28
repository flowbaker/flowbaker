package claudeintegration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/rs/zerolog/log"
)

const (
	ClaudeIntegrationActionType_Prompt    domain.IntegrationActionType   = "prompt"
	ClaudeIntegrationActionType_AgentChat domain.IntegrationActionType   = "ai_agent_chat" // Added for agent LLM
	ClaudeIntegrationPeekable_Models      domain.IntegrationPeekableType = "models"
)

type ClaudeIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[ClaudeCredential]
}

func NewClaudeIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &ClaudeIntegrationCreator{
		binder:           deps.ParameterBinder,
		credentialGetter: managers.NewExecutorCredentialGetter[ClaudeCredential](deps.ExecutorCredentialManager),
	}
}

func (c *ClaudeIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewClaudeIntegration(ctx, ClaudeIntegrationDependencies{
		CredentialID:     p.CredentialID,
		ParameterBinder:  c.binder,
		CredentialGetter: c.credentialGetter,
	})
}

type ClaudeIntegration struct {
	client           anthropic.Client
	binder           domain.IntegrationParameterBinder
	credentialID     string // Add this field
	credentialGetter domain.CredentialGetter[ClaudeCredential]

	actionManager *domain.IntegrationActionManager
	peekFuncs     map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type ClaudeCredential struct {
	APIKey string `json:"api_key"`
}

type ClaudeIntegrationDependencies struct {
	CredentialID     string
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[ClaudeCredential]
}

func NewClaudeIntegration(ctx context.Context, deps ClaudeIntegrationDependencies) (*ClaudeIntegration, error) {
	integration := &ClaudeIntegration{
		binder:           deps.ParameterBinder,
		credentialGetter: deps.CredentialGetter,
		credentialID:     deps.CredentialID, // Set the credential ID
		actionManager:    domain.NewIntegrationActionManager(),
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(ClaudeIntegrationActionType_Prompt, integration.Prompt).
		Add(ClaudeIntegrationActionType_AgentChat, integration.AgentChat) // Register agent chat handler

	integration.actionManager = actionManager

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		ClaudeIntegrationPeekable_Models: integration.PeekModels,
	}

	integration.peekFuncs = peekFuncs

	// Get API key from credentials
	credential, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, err
	}

	// Initialize Anthropic client
	integration.client = anthropic.NewClient(
		option.WithAPIKey(credential.APIKey),
	)

	return integration, nil
}

func (i *ClaudeIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type PromptParams struct {
	Model        string  `json:"model"`
	Prompt       string  `json:"prompt"`
	MaxTokens    int     `json:"max_tokens"`
	Temperature  float64 `json:"temperature,omitempty"`
	SystemPrompt string  `json:"system_prompt,omitempty"`
}

type PromptOutputItem struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	Usage        Usage  `json:"usage"`
	FinishReason string `json:"finish_reason"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (i *ClaudeIntegration) Prompt(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := PromptParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Call Claude API using SDK
	output, err := i.sendPrompt(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("failed to call Claude API: %w", err)
	}

	return output, nil
}

// sendPrompt makes the API call to Claude using the official SDK
func (i *ClaudeIntegration) sendPrompt(ctx context.Context, params PromptParams) (PromptOutputItem, error) {
	// Set default max_tokens if not specified
	maxTokens := int64(params.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = 1024 // Default value
	}

	// Prepare message parameters using the correct SDK structure
	messageParams := anthropic.MessageNewParams{
		Model:     anthropic.Model(params.Model),
		MaxTokens: maxTokens,
		Messages: []anthropic.MessageParam{{
			Content: []anthropic.ContentBlockParamUnion{{
				OfText: &anthropic.TextBlockParam{Text: params.Prompt},
			}},
			Role: anthropic.MessageParamRoleUser,
		}},
	}

	// Add system prompt if provided
	if params.SystemPrompt != "" {
		messageParams.System = []anthropic.TextBlockParam{{
			Text: params.SystemPrompt,
		}}
	}

	// Add temperature if provided
	if params.Temperature > 0 {
		messageParams.Temperature = anthropic.Float(params.Temperature)
	}

	// Make API call
	message, err := i.client.Messages.New(ctx, messageParams)
	if err != nil {
		return PromptOutputItem{}, fmt.Errorf("failed to create message: %w", err)
	}

	// Extract content from response
	content := ""
	if len(message.Content) > 0 {
		switch block := message.Content[0].AsAny().(type) {
		case anthropic.TextBlock:
			content = block.Text
		}
	}

	// Convert usage information
	usage := Usage{
		InputTokens:  int(message.Usage.InputTokens),
		OutputTokens: int(message.Usage.OutputTokens),
	}

	output := PromptOutputItem{
		Content:      content,
		Model:        string(message.Model),
		Usage:        usage,
		FinishReason: string(message.StopReason),
	}

	return output, nil
}

func (i *ClaudeIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx, params)
}

func (i *ClaudeIntegration) PeekModels(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	// Get available models from Anthropic API
	models, err := i.getModelsAPI(ctx)
	if err != nil {
		// If API call fails, return default models as fallback
		log.Warn().Err(err).Msg("Failed to fetch models from API, using fallback list")
		models = i.getDefaultModels()
	}

	var results []domain.PeekResultItem
	for _, model := range models {
		results = append(results, domain.PeekResultItem{
			Key:     model.ID,
			Value:   model.ID,
			Content: model.DisplayName,
		})
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

// ClaudeModel represents a Claude model
type ClaudeModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

// getModelsAPI gets models from Anthropic API using the SDK
func (i *ClaudeIntegration) getModelsAPI(ctx context.Context) ([]ClaudeModel, error) {
	// Use the SDK to get models
	modelsList, err := i.client.Models.List(ctx, anthropic.ModelListParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	// Convert SDK models to our model format
	var models []ClaudeModel
	for _, model := range modelsList.Data {
		models = append(models, ClaudeModel{
			ID:          string(model.ID),
			DisplayName: model.DisplayName,
			Type:        string(model.Type),
		})
	}

	// If no models returned from API, use fallback
	if len(models) == 0 {
		return i.getDefaultModels(), nil
	}

	return models, nil
}

// getDefaultModels returns the current available Claude models as fallback
func (i *ClaudeIntegration) getDefaultModels() []ClaudeModel {
	return []ClaudeModel{
		{
			ID:          "claude-opus-4-5-20251101",
			DisplayName: "Claude Opus 4.5",
			Type:        "model",
		},
		{
			ID:          "claude-haiku-4-5-20251001",
			DisplayName: "Claude Haiku 4.5",
			Type:        "model",
		},
		{
			ID:          "claude-sonnet-4-5-20250929",
			DisplayName: "Claude Sonnet 4.5",
			Type:        "model",
		},
		{
			ID:          "claude-opus-4-1-20250805",
			DisplayName: "Claude Opus 4.1",
			Type:        "model",
		},
		{
			ID:          "claude-opus-4-20250514",
			DisplayName: "Claude Opus 4",
			Type:        "model",
		},
		{
			ID:          "claude-sonnet-4-20250514",
			DisplayName: "Claude Sonnet 4",
			Type:        "model",
		},
		{
			ID:          "claude-3-7-sonnet-20250219",
			DisplayName: "Claude Sonnet 3.7",
			Type:        "model",
		},
		{
			ID:          "claude-3-5-haiku-20241022",
			DisplayName: "Claude Haiku 3.5",
			Type:        "model",
		},
		{
			ID:          "claude-3-haiku-20240307",
			DisplayName: "Claude Haiku 3",
			Type:        "model",
		},
		{
			ID:          "claude-3-opus-20240229",
			DisplayName: "Claude Opus 3",
			Type:        "model",
		},
	}
}

// ClaudeAPIRequest represents the raw request to Claude API
type ClaudeAPIRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []ClaudeAPIMessage `json:"messages"`
	Tools     []ClaudeAPITool    `json:"tools,omitempty"`
	System    string             `json:"system,omitempty"`
}

// ClaudeAPIMessage represents a message in Claude API format
type ClaudeAPIMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// ClaudeAPITool represents a tool in Claude API format
type ClaudeAPITool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ClaudeAPIResponse represents the response from Claude API
type ClaudeAPIResponse struct {
	Content []ClaudeContentBlock `json:"content"`
	Model   string               `json:"model"`
	Usage   ClaudeUsage          `json:"usage"`
}

// ClaudeContentBlock represents a content block in Claude response
type ClaudeContentBlock struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

// ClaudeUsage represents usage statistics
type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AgentChatParams represents parameters for AI agent chat requests
type AgentChatParams struct {
	Model        string             `json:"model"`
	Messages     []ClaudeAPIMessage `json:"messages"`
	Tools        []ClaudeAPITool    `json:"tools,omitempty"`
	SystemPrompt string             `json:"system_prompt,omitempty"`
	MaxTokens    int                `json:"max_tokens"`
	Temperature  float64            `json:"temperature,omitempty"`
}

// AgentChatOutput represents the output from agent chat
type AgentChatOutput struct {
	Content    []ClaudeContentBlock `json:"content"`
	Model      string               `json:"model"`
	Usage      ClaudeUsage          `json:"usage"`
	StopReason string               `json:"stop_reason"`
}

// Add the AgentChat handler implementation
func (i *ClaudeIntegration) AgentChat(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Executing AI agent chat with Claude")

	// Get all items from the input
	items, err := params.GetAllItems()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	outputItems := make([]any, 0, len(items))

	for _, item := range items {
		p := AgentChatParams{}

		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		// Set default max_tokens if not specified
		maxTokens := int64(p.MaxTokens)
		if maxTokens <= 0 {
			maxTokens = 1024
		}

		// Convert messages to Claude SDK format
		var claudeMessages []anthropic.MessageParam
		for _, msg := range p.Messages {
			claudeMsg := anthropic.MessageParam{
				Role: anthropic.MessageParamRole(msg.Role),
			}

			// Handle different content types
			switch content := msg.Content.(type) {
			case string:
				if content != "" {
					claudeMsg.Content = []anthropic.ContentBlockParamUnion{{
						OfText: &anthropic.TextBlockParam{Text: content},
					}}
				}
			case []interface{}:
				// Handle structured content (text + tool calls)
				var contentBlocks []anthropic.ContentBlockParamUnion
				for _, block := range content {
					if blockMap, ok := block.(map[string]interface{}); ok {
						blockType, _ := blockMap["type"].(string)
						switch blockType {
						case "text":
							if text, ok := blockMap["text"].(string); ok && text != "" {
								contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
									OfText: &anthropic.TextBlockParam{Text: text},
								})
							}
						case "tool_use":
							// Handle tool use blocks
							toolUseBlock := anthropic.ToolUseBlockParam{
								ID:   blockMap["id"].(string),
								Name: blockMap["name"].(string),
							}
							if input, ok := blockMap["input"].(map[string]interface{}); ok {
								toolUseBlock.Input = input
							}
							contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
								OfToolUse: &toolUseBlock,
							})
						case "tool_result":
							// Handle tool result blocks
							toolResultBlock := anthropic.ToolResultBlockParam{
								ToolUseID: blockMap["tool_use_id"].(string),
							}
							if content, ok := blockMap["content"].(string); ok && content != "" {
								toolResultBlock.Content = []anthropic.ToolResultBlockParamContentUnion{{
									OfText: &anthropic.TextBlockParam{Text: content},
								}}
							}
							contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
								OfToolResult: &toolResultBlock,
							})
						}
					}
				}
				if len(contentBlocks) > 0 {
					claudeMsg.Content = contentBlocks
				}
			}

			// Only add message if it has content
			if len(claudeMsg.Content) > 0 {
				claudeMessages = append(claudeMessages, claudeMsg)
			}
		}

		// Prepare message parameters
		messageParams := anthropic.MessageNewParams{
			Model:     anthropic.Model(p.Model),
			MaxTokens: maxTokens,
			Messages:  claudeMessages,
		}

		// Add system prompt if provided
		if p.SystemPrompt != "" {
			messageParams.System = []anthropic.TextBlockParam{{
				Text: p.SystemPrompt,
			}}
		}

		// Add temperature if provided
		if p.Temperature > 0 {
			messageParams.Temperature = anthropic.Float(p.Temperature)
		}

		// Add native tools support
		if len(p.Tools) > 0 {
			var claudeTools []anthropic.ToolUnionParam
			for _, tool := range p.Tools {
				// Convert input schema to the required format
				inputSchemaJSON, err := json.Marshal(tool.InputSchema)
				if err != nil {
					continue
				}

				var inputSchemaParam anthropic.ToolInputSchemaParam
				if err := json.Unmarshal(inputSchemaJSON, &inputSchemaParam); err != nil {
					continue
				}

				claudeTool := anthropic.ToolUnionParam{
					OfTool: &anthropic.ToolParam{
						Name:        tool.Name,
						Description: anthropic.String(tool.Description),
						InputSchema: inputSchemaParam,
					},
				}
				claudeTools = append(claudeTools, claudeTool)
			}
			messageParams.Tools = claudeTools
		}

		// Make API call
		message, err := i.client.Messages.New(ctx, messageParams)
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to call Claude API: %w", err)
		}

		// Convert response to our format
		output := AgentChatOutput{
			Model:      string(message.Model),
			StopReason: string(message.StopReason),
			Usage: ClaudeUsage{
				InputTokens:  int(message.Usage.InputTokens),
				OutputTokens: int(message.Usage.OutputTokens),
			},
		}

		// Convert content blocks
		for _, block := range message.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				output.Content = append(output.Content, ClaudeContentBlock{
					Type: "text",
					Text: b.Text,
				})
			case anthropic.ToolUseBlock:
				// Convert RawMessage to map[string]interface{}
				var input map[string]interface{}
				if err := json.Unmarshal(b.Input, &input); err != nil {
					input = make(map[string]interface{})
				}

				output.Content = append(output.Content, ClaudeContentBlock{
					Type:  "tool_use",
					ID:    b.ID,
					Name:  b.Name,
					Input: input,
				})
			}
		}

		outputItems = append(outputItems, output)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	log.Info().Msgf("Agent chat result: %s", string(resultJSON))

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

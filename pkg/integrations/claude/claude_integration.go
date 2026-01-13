package claudeintegration

import (
	"context"
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
		AddPerItem(ClaudeIntegrationActionType_Prompt, integration.Prompt)

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

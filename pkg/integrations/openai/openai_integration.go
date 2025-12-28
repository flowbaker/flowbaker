package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/sashabaranov/go-openai"
)

const (
	IntegrationActionType_Chat              domain.IntegrationActionType = "chat"
	IntegrationActionType_GenerateImage     domain.IntegrationActionType = "generate_image"
	IntegrationActionType_AgentChat         domain.IntegrationActionType = "ai_agent_chat" // Added for agent LLM
	IntegrationActionType_GenerateEmbedding domain.IntegrationActionType = "generate_embeddings"
)

const (
	OpenAIIntegrationPeekable_Models domain.IntegrationPeekableType = "models"
)

type OpenAIIntegrationCreator struct {
	credentialGetter       domain.CredentialGetter[OpenAICredential]
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
}

func NewOpenAIIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &OpenAIIntegrationCreator{
		credentialGetter:       managers.NewExecutorCredentialGetter[OpenAICredential](deps.ExecutorCredentialManager),
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}
}

func (c *OpenAIIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	deps := OpenAIIntegrationDependencies{
		CredentialGetter:       c.credentialGetter,
		ParameterBinder:        c.binder,
		CredentialID:           p.CredentialID,
		ExecutorStorageManager: c.executorStorageManager,
		WorkspaceID:            p.WorkspaceID,
	}

	return NewOpenAIIntegration(ctx, deps)
}

type OpenAIIntegration struct {
	credentialGetter domain.CredentialGetter[OpenAICredential]
	binder           domain.IntegrationParameterBinder
	client           *openai.Client

	executorStorageManager domain.ExecutorStorageManager
	workspaceID            string
	actionManager          *domain.IntegrationActionManager
	peekFuncs              map[domain.IntegrationPeekableType]func(ctx context.Context) (domain.PeekResult, error)
}

type OpenAICredential struct {
	APIKey string `json:"api_key"`
}

type OpenAIIntegrationDependencies struct {
	CredentialID           string
	CredentialGetter       domain.CredentialGetter[OpenAICredential]
	ParameterBinder        domain.IntegrationParameterBinder
	WorkspaceID            string
	ExecutorStorageManager domain.ExecutorStorageManager
}

func NewOpenAIIntegration(ctx context.Context, deps OpenAIIntegrationDependencies) (*OpenAIIntegration, error) {
	integration := &OpenAIIntegration{
		credentialGetter:       deps.CredentialGetter,
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
		workspaceID:            deps.WorkspaceID,
	}

	integration.actionManager = domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_Chat, integration.ChatCompletion).
		AddPerItem(IntegrationActionType_GenerateImage, integration.GenerateImage).
		AddPerItem(IntegrationActionType_AgentChat, integration.AgentChat).
		AddPerItem(IntegrationActionType_GenerateEmbedding, integration.GenerateEmbeddings)

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context) (domain.PeekResult, error){
		OpenAIIntegrationPeekable_Models: integration.PeekModels,
	}

	integration.peekFuncs = peekFuncs

	if integration.client == nil {
		credential, err := integration.credentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
		if err != nil {
			return nil, err
		}

		integration.client = openai.NewClient(credential.APIKey)
	}

	return integration, nil
}

func (i *OpenAIIntegration) SetClient(client *openai.Client) {
	i.client = client
}

func (i *OpenAIIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Executing OpenAI integration")

	return i.actionManager.Run(ctx, params.ActionType, params)
}

type ChatCompletionParams struct {
	Model               string                         `json:"model"`
	Messages            []openai.ChatCompletionMessage `json:"messages"`
	Temperature         float32                        `json:"temperature"`
	MaxTokens           int                            `json:"max_tokens"`
	MaxCompletionTokens int                            `json:"max_completion_tokens"`
}

func (i *OpenAIIntegration) ChatCompletion(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ChatCompletionParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	req := openai.ChatCompletionRequest{
		Model:       p.Model,
		Messages:    p.Messages,
		Temperature: p.Temperature,
	}

	if isMaxCompletionTokensModel(p.Model) {
		req.MaxCompletionTokens = p.MaxCompletionTokens
	} else {
		req.MaxTokens = p.MaxTokens
	}

	resp, err := i.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type ImageGenerationParams struct {
	Prompt  string `json:"prompt"`
	Size    string `json:"size"`
	Model   string `json:"model"`
	Quality string `json:"quality"` // "standard" or "hd"
	Count   int    `json:"count"`
	Style   string `json:"style"` // "vivid" or "natural"

}

func (i *OpenAIIntegration) GenerateImage(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ImageGenerationParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.Model == "dall-e-3" && p.Count > 1 {
		return nil, fmt.Errorf("dall-e-3 does not support multiple images")
	}

	if p.Model == "dall-e-2" && p.Style != "" {
		return nil, fmt.Errorf("style is not supported for dall-e-2")
	}

	resp, err := i.client.CreateImage(ctx, openai.ImageRequest{
		Prompt:         p.Prompt,
		Size:           p.Size,
		Model:          p.Model,
		Quality:        p.Quality,
		ResponseFormat: openai.CreateImageResponseFormatB64JSON,
		N:              p.Count,
		Style:          p.Style,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create image")
		return nil, err
	}

	fileItems := []domain.FileItem{}

	for index := range resp.Data {
		imageData, err := base64.StdEncoding.DecodeString(resp.Data[index].B64JSON)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode image data")
			return nil, err
		}

		putParams := domain.PutExecutionFileParams{
			WorkspaceID:  i.workspaceID,
			UploadedBy:   i.workspaceID,
			OriginalName: uuid.NewString(),
			SizeInBytes:  int64(len(imageData)),
			ContentType:  "image/png",
			Reader:       io.NopCloser(bytes.NewReader(imageData)),
		}

		fileItem, err := i.executorStorageManager.PutExecutionFile(ctx, putParams)
		if err != nil {
			log.Error().Err(err).Msg("Failed to put image to object storage")
		}

		fileItems = append(fileItems, fileItem)
		resp.Data[index].B64JSON = ""
	}

	respJson, err := json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal response")
		return nil, err
	}

	var resultMap map[string]any

	err = json.Unmarshal(respJson, &resultMap)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal response")
		return nil, err
	}

	resultMap["files"] = fileItems

	return resultMap, nil
}

func (i *OpenAIIntegration) ListModels(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	models, err := i.client.ListModels(ctx)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	resultJSON, err := json.Marshal(models.Models)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (i *OpenAIIntegration) Peek(ctx context.Context, params domain.IntegrationPeekableType) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx)
}

func (i *OpenAIIntegration) PeekModels(ctx context.Context) (domain.PeekResult, error) {
	models, err := i.client.ListModels(ctx)
	if err != nil {
		return domain.PeekResult{}, err
	}

	var results []domain.PeekResultItem
	for _, model := range models.Models {
		results = append(results, domain.PeekResultItem{
			Key:     model.ID,
			Value:   model.ID,
			Content: model.ID,
		})
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

// AgentChatParams represents parameters for AI agent chat requests
type AgentChatParams struct {
	Model               string                         `json:"model"`
	Messages            []openai.ChatCompletionMessage `json:"messages"`
	Tools               []openai.Tool                  `json:"tools,omitempty"`
	SystemPrompt        string                         `json:"system_prompt,omitempty"`
	Temperature         float32                        `json:"temperature"`
	MaxTokens           int                            `json:"max_tokens"`
	MaxCompletionTokens int                            `json:"max_completion_tokens"`
}

// Add the AgentChat handler implementation
func (i *OpenAIIntegration) AgentChat(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	log.Info().Msgf("Executing AI agent chat with OpenAI")

	p := AgentChatParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	// Add system prompt to messages if provided
	messages := p.Messages
	if p.SystemPrompt != "" {
		// Prepend system message
		systemMsg := openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: p.SystemPrompt,
		}
		messages = append([]openai.ChatCompletionMessage{systemMsg}, messages...)
	}

	req := openai.ChatCompletionRequest{
		Model:       p.Model,
		Messages:    messages,
		Temperature: p.Temperature,
		Tools:       p.Tools,
	}

	if isMaxCompletionTokensModel(p.Model) {
		req.MaxCompletionTokens = p.MaxCompletionTokens
	} else {
		req.MaxTokens = p.MaxTokens
	}

	resp, err := i.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return domain.IntegrationOutput{}, fmt.Errorf("failed to call OpenAI API: %w", err)
	}

	return resp, nil
}

type GenerateEmbeddingsParams struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

var maxCompletionTokensModels = map[string]bool{
	// O1 models
	"o1":            true,
	"o1-2024-12-17": true,
	// O2 models
	"o2":         true,
	"o2-mini":    true,
	"o2-preview": true,
	// O3 models
	"o3":                 true,
	"o3-2025-04-16":      true,
	"o3-mini":            true,
	"o3-mini-2025-01-31": true,
	// O4 models
	"o4":                 true,
	"o4-mini":            true,
	"o4-mini-2025-04-16": true,
	// GPT-5 models
	"gpt-5":             true,
	"gpt-5-mini":        true,
	"gpt-5-nano":        true,
	"gpt-5-chat-latest": true,
}

func isMaxCompletionTokensModel(model string) bool {
	return maxCompletionTokensModels[model]
}

func (i *OpenAIIntegration) GenerateEmbeddings(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {

	p := GenerateEmbeddingsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.Model == "" {
		p.Model = "text-embedding-3-small"
	}

	if len(p.Input) == 0 {
		return nil, fmt.Errorf("no input provided for embedding generation")
	}

	req := openai.EmbeddingRequest{
		Input: p.Input,
		Model: openai.EmbeddingModel(p.Model),
	}

	resp, err := i.client.CreateEmbeddings(ctx, req)
	if err != nil {
		log.Error().
			Err(err).
			Str("model", p.Model).
			Int("input_count", len(p.Input)).
			Msg("Failed to generate embeddings")
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for idx, data := range resp.Data {
		embeddings[idx] = data.Embedding
	}

	result := map[string]interface{}{
		"model":      p.Model,
		"embeddings": embeddings,
		"usage": map[string]interface{}{
			"prompt_tokens": resp.Usage.PromptTokens,
			"total_tokens":  resp.Usage.TotalTokens,
		},
	}

	return result, nil
}

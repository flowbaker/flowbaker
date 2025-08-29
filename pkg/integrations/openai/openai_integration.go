package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"
	"io"

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
	Model       string                         `json:"model"`
	Messages    []openai.ChatCompletionMessage `json:"messages"`
	Temperature float32                        `json:"temperature"`
	MaxTokens   int                            `json:"max_tokens"`
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
		MaxTokens:   p.MaxTokens,
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

	resultJSON, err := json.Marshal(models.Models)
	if err != nil {
		return domain.PeekResult{}, err
	}

	return domain.PeekResult{
		ResultJSON: resultJSON,
	}, nil
}

// AgentChatParams represents parameters for AI agent chat requests
type AgentChatParams struct {
	Model        string                         `json:"model"`
	Messages     []openai.ChatCompletionMessage `json:"messages"`
	Tools        []openai.Tool                  `json:"tools,omitempty"`
	SystemPrompt string                         `json:"system_prompt,omitempty"`
	Temperature  float32                        `json:"temperature"`
	MaxTokens    int                            `json:"max_tokens"`
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
		MaxTokens:   p.MaxTokens,
		Tools:       p.Tools,
	}

	resp, err := i.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return domain.IntegrationOutput{}, fmt.Errorf("failed to call OpenAI API: %w", err)
	}

	return resp, nil
}

// GenerateWithConversation implements domain.IntegrationLLM interface
func (i *OpenAIIntegration) GenerateWithConversation(ctx context.Context, req domain.GenerateRequest) (domain.ModelResponse, error) {
	// Convert domain messages to OpenAI messages
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages)+1)

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: req.SystemPrompt,
		})
	}

	// Convert conversation messages
	for _, msg := range req.Messages {
		openaiMsg := openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]openai.ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Arguments)
				toolCalls[i] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				}
			}
			openaiMsg.ToolCalls = toolCalls
		}

		// Handle tool results (convert to tool role messages)
		if len(msg.ToolResults) > 0 {
			for _, result := range msg.ToolResults {
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    result.Content,
					ToolCallID: result.ToolCallID,
				})
			}
		} else {
			messages = append(messages, openaiMsg)
		}
	}

	// Convert domain tools to OpenAI tools
	var tools []openai.Tool
	if len(req.Tools) > 0 {
		tools = make([]openai.Tool, len(req.Tools))
		for i, tool := range req.Tools {
			tools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			}
		}
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000
	}

	openaiReq := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Tools:       tools,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	resp, err := i.client.CreateChatCompletion(ctx, openaiReq)
	if err != nil {
		return domain.ModelResponse{}, fmt.Errorf("failed to call OpenAI API: %w", err)
	}

	if len(resp.Choices) == 0 {
		return domain.ModelResponse{}, fmt.Errorf("no choices in OpenAI response")
	}

	choice := resp.Choices[0]
	response := domain.ModelResponse{
		Content:      choice.Message.Content,
		FinishReason: string(choice.FinishReason),
	}

	if len(choice.Message.ToolCalls) > 0 {
		toolCalls := make([]domain.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			var args map[string]interface{}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					log.Error().Err(err).Msg("Failed to unmarshal tool call arguments")
					args = make(map[string]interface{})
				}
			}

			toolCalls[i] = domain.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			}
		}
		response.ToolCalls = toolCalls
	}

	return response, nil
}

type GenerateEmbeddingsParams struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
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

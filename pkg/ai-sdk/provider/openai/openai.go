package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/rs/zerolog/log"
	"github.com/sashabaranov/go-openai"
)

// Provider implements the LanguageModel interface for OpenAI
type Provider struct {
	client  *openai.Client
	apiKey  string
	baseURL string

	RequestSettings RequestSettings
}

// Option is a function that configures the Provider
type Option func(*Provider)

// WithBaseURL sets a custom base URL (for OpenAI-compatible APIs like Groq)
func WithBaseURL(baseURL string) Option {
	return func(p *Provider) {
		p.baseURL = baseURL
	}
}

type RequestSettings struct {
	Model            string
	Temperature      float32
	MaxTokens        int
	TopP             float32
	FrequencyPenalty float32
	PresencePenalty  float32
	Stop             []string
	ReasoningEffort  string
	Verbosity        string
}

// New creates a new OpenAI provider
func New(apiKey, model string, opts ...Option) *Provider {
	p := &Provider{
		apiKey: apiKey,
		RequestSettings: RequestSettings{
			Model: model,
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(p)
	}

	// Create client with optional custom base URL
	clientConfig := openai.DefaultConfig(apiKey)
	if p.baseURL != "" {
		clientConfig.BaseURL = p.baseURL
	}
	p.client = openai.NewClientWithConfig(clientConfig)

	return p
}

func (p *Provider) SetRequestSettings(settings RequestSettings) {
	p.RequestSettings = settings
}

// Generate implements the Generate method of the LanguageModel interface
func (p *Provider) Generate(ctx context.Context, req provider.GenerateRequest) (*types.GenerateResponse, error) {
	messages := p.convertMessages(req.Messages, req.System)
	tools := p.convertTools(req.Tools)

	chatReq := openai.ChatCompletionRequest{
		Model:    p.RequestSettings.Model,
		Messages: messages,
		Tools:    tools,

		Verbosity:        p.RequestSettings.Verbosity,
		ReasoningEffort:  p.RequestSettings.ReasoningEffort,
		Temperature:      p.RequestSettings.Temperature,
		TopP:             p.RequestSettings.TopP,
		FrequencyPenalty: p.RequestSettings.FrequencyPenalty,
		PresencePenalty:  p.RequestSettings.PresencePenalty,
		Stop:             p.RequestSettings.Stop,
	}

	if p.RequestSettings.MaxTokens > 0 {
		if isMaxCompletionTokensModel(p.RequestSettings.Model) {
			chatReq.MaxCompletionTokens = p.RequestSettings.MaxTokens
		} else {
			chatReq.MaxTokens = p.RequestSettings.MaxTokens
		}
	}

	resp, err := p.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		log.Error().Err(err).Str("model", p.RequestSettings.Model).Msg("openai api error")
		return nil, fmt.Errorf("openai api error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, types.ErrEmptyResponse
	}

	choice := resp.Choices[0]
	response := &types.GenerateResponse{
		Content:      choice.Message.Content,
		FinishReason: string(choice.FinishReason),
		Model:        resp.Model,
		Usage: types.Usage{
			PromptTokens:      resp.Usage.PromptTokens,
			CompletionTokens:  resp.Usage.CompletionTokens,
			TotalTokens:       resp.Usage.TotalTokens,
			ReasoningTokens:   resp.Usage.CompletionTokensDetails.ReasoningTokens,
			CachedInputTokens: resp.Usage.PromptTokensDetails.CachedTokens,
		},
	}

	// Convert tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		response.ToolCalls = make([]types.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			var args map[string]any
			if tc.Function.Arguments != "" {
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
			}
			response.ToolCalls[i] = types.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			}
		}
	}

	return response, nil
}

func (p *Provider) Stream(ctx context.Context, req provider.GenerateRequest) (*provider.ProviderStream, error) {
	messages := p.convertMessages(req.Messages, req.System)
	tools := p.convertTools(req.Tools)

	chatReq := openai.ChatCompletionRequest{
		Model:    p.RequestSettings.Model,
		Messages: messages,
		Stream:   true,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
		Verbosity:        p.RequestSettings.Verbosity,
		ReasoningEffort:  p.RequestSettings.ReasoningEffort,
		Temperature:      p.RequestSettings.Temperature,
		TopP:             p.RequestSettings.TopP,
		FrequencyPenalty: p.RequestSettings.FrequencyPenalty,
		PresencePenalty:  p.RequestSettings.PresencePenalty,
		Stop:             p.RequestSettings.Stop,
		Tools:            tools,
	}

	if p.RequestSettings.MaxTokens > 0 {
		if isMaxCompletionTokensModel(p.RequestSettings.Model) {
			chatReq.MaxCompletionTokens = p.RequestSettings.MaxTokens
		} else {
			chatReq.MaxTokens = p.RequestSettings.MaxTokens
		}
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, chatReq)
	if err != nil {
		log.Error().Err(err).Str("model", p.RequestSettings.Model).Msg("openai stream error")
		return nil, fmt.Errorf("openai stream error: %w", err)
	}

	eventChan := make(chan types.StreamEvent, 100)
	ps := provider.NewProviderStream(eventChan)

	go func() {
		defer close(eventChan)
		defer stream.Close()

		toolCallsMap := make(map[int]*toolCallBuilder)
		var totalUsage types.Usage
		var fullText string
		var modelID string
		var systemFingerprint string
		streamStarted := false

		for {
			response, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				ps.SetError(fmt.Errorf("stream recv error: %w", err))
				return
			}

			if !streamStarted {
				modelID = response.Model
				systemFingerprint = response.SystemFingerprint
				eventChan <- types.NewStreamStartEvent(modelID, response.ID, systemFingerprint)
				streamStarted = true
			}

			if response.Usage != nil {
				usage := types.Usage{
					PromptTokens:     response.Usage.PromptTokens,
					CompletionTokens: response.Usage.CompletionTokens,
					TotalTokens:      response.Usage.TotalTokens,
				}

				if response.Usage.CompletionTokensDetails != nil {
					usage.ReasoningTokens = response.Usage.CompletionTokensDetails.ReasoningTokens
				}
				if response.Usage.PromptTokensDetails != nil {
					usage.CachedInputTokens = response.Usage.PromptTokensDetails.CachedTokens
				}

				totalUsage = usage
				eventChan <- types.NewUsageEvent(totalUsage)
			}

			if len(response.Choices) == 0 {
				continue
			}

			choice := response.Choices[0]
			delta := choice.Delta

			if delta.Content != "" {
				fullText += delta.Content
				eventChan <- types.NewTextDeltaEvent(delta.Content, choice.Index)
			}

			if len(delta.ToolCalls) > 0 {
				for _, tc := range delta.ToolCalls {
					if tc.Index == nil {
						continue
					}
					index := *tc.Index
					if _, exists := toolCallsMap[index]; !exists {
						toolCallsMap[index] = &toolCallBuilder{
							id:        tc.ID,
							name:      tc.Function.Name,
							arguments: "",
						}
						if tc.ID != "" {
							eventChan <- types.NewToolCallStartEvent(tc.ID, tc.Function.Name, index)
						}
					}

					if tc.Function.Arguments != "" {
						toolCallsMap[index].arguments += tc.Function.Arguments
						eventChan <- types.NewToolCallDeltaEvent(
							toolCallsMap[index].id,
							tc.Function.Arguments,
							index,
						)
					}
				}
			}

			if choice.FinishReason != "" {
				eventChan <- types.NewFinishReasonEvent(string(choice.FinishReason))
			}
		}

		if fullText != "" {
			eventChan <- types.NewTextCompleteEvent(fullText)
		}

		for index, builder := range toolCallsMap {
			var args map[string]any
			if builder.arguments != "" {
				json.Unmarshal([]byte(builder.arguments), &args)
			}
			toolCall := types.ToolCall{
				ID:        builder.id,
				Name:      builder.name,
				Arguments: args,
			}
			eventChan <- types.NewToolCallCompleteEvent(toolCall, index)
		}

		eventChan <- types.NewStreamEndEvent("stop", totalUsage)
	}()

	return ps, nil
}

// Helper type for building tool calls from deltas
type toolCallBuilder struct {
	id        string
	name      string
	arguments string
}

// ID returns the model identifier
func (p *Provider) ID() string {
	return fmt.Sprintf("openai:%s", p.RequestSettings.Model)
}

// Capabilities returns the model's capabilities
func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsTools:     true,
		SupportsStreaming: true,
		SupportsVision:    isVisionModel(p.RequestSettings.Model),
		MaxContextTokens:  getMaxContextTokens(p.RequestSettings.Model),
		MaxOutputTokens:   getMaxOutputTokens(p.RequestSettings.Model),
	}
}

// Helper functions
func (p *Provider) convertMessages(messages []types.Message, system string) []openai.ChatCompletionMessage {
	result := make([]openai.ChatCompletionMessage, 0, len(messages)+1)

	// Add system message if provided
	if system != "" {
		result = append(result, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: system,
		})
	}

	// Convert messages
	for _, msg := range messages {
		oaiMsg := openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}

		// Convert tool calls
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
			oaiMsg.ToolCalls = toolCalls
		}

		// Handle tool results - convert to separate tool messages
		if len(msg.ToolResults) > 0 {
			// First add the assistant message with tool calls if present
			if len(msg.ToolCalls) > 0 {
				result = append(result, oaiMsg)
			}
			// Then add tool result messages
			for _, toolResult := range msg.ToolResults {
				result = append(result, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    toolResult.Content,
					ToolCallID: toolResult.ToolCallID,
				})
			}
		} else {
			result = append(result, oaiMsg)
		}
	}

	return result
}

func (p *Provider) convertTools(tools []types.Tool) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}

	result := make([]openai.Tool, len(tools))
	for i, tool := range tools {
		result[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		}
	}
	return result
}

// Model capability helpers
var maxCompletionTokensModels = map[string]bool{
	"o1": true, "o1-2024-12-17": true, "o1-mini": true, "o1-mini-2024-09-12": true,
	"o1-preview": true, "o1-preview-2024-09-12": true,
	"o3": true, "o3-mini": true,
	"gpt-5": true, "gpt-5-mini": true, "gpt-5-nano": true,
	"gpt-5-chat-latest":   true,
	"gpt-5.2-chat-latest": true,
}

func isMaxCompletionTokensModel(model string) bool {
	return maxCompletionTokensModels[model]
}

func isVisionModel(model string) bool {
	visionModels := map[string]bool{
		"gpt-4-vision-preview": true, "gpt-4-turbo": true,
		"gpt-4o": true, "gpt-4o-mini": true,
	}
	return visionModels[model]
}

func getMaxContextTokens(model string) int {
	contextLimits := map[string]int{
		"gpt-5":         400000,
		"gpt-5-mini":    400000,
		"gpt-5-nano":    400000,
		"gpt-4":         8192,
		"gpt-4-32k":     32768,
		"gpt-4o":        128000,
		"gpt-4o-mini":   128000,
		"gpt-3.5-turbo": 16385,
	}
	if limit, ok := contextLimits[model]; ok {
		return limit
	}
	return 8192 // default
}

func getMaxOutputTokens(model string) int {
	outputLimits := map[string]int{
		"gpt-5":         128000,
		"gpt-5-mini":    128000,
		"gpt-5-nano":    128000,
		"gpt-4":         4096,
		"gpt-4o":        4096,
		"gpt-4o-mini":   16384,
		"gpt-3.5-turbo": 4096,
	}
	if limit, ok := outputLimits[model]; ok {
		return limit
	}
	return 4096 // default
}

func (p *Provider) ProviderName() string {
	return "openai"
}

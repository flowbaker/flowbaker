package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Provider implements the LanguageModel interface for Anthropic
type Provider struct {
	client anthropic.Client
	apiKey string

	RequestSettings RequestSettings
}

type RequestSettings struct {
	Model       string
	MaxTokens   int64
	Temperature float64
	TopP        float64
	TopK        int64
	Stop        []string
}

// New creates a new Anthropic provider
func New(apiKey, model string) *Provider {
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &Provider{
		client: client,
		apiKey: apiKey,
		RequestSettings: RequestSettings{
			Model:     model,
			MaxTokens: 4096, // Default max tokens for Anthropic
		},
	}
}

func (p *Provider) SetRequestSettings(settings RequestSettings) {
	p.RequestSettings = settings
}

// Generate implements the Generate method of the LanguageModel interface
func (p *Provider) Generate(ctx context.Context, req provider.GenerateRequest) (*types.GenerateResponse, error) {
	messages := p.convertMessages(req.Messages)
	tools := p.convertTools(req.Tools)

	messageParams := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.RequestSettings.Model),
		MaxTokens: p.RequestSettings.MaxTokens,
		Messages:  messages,
	}

	// Add system prompt if provided
	if req.System != "" {
		messageParams.System = []anthropic.TextBlockParam{{
			Text: req.System,
		}}
	}

	// Add temperature if set
	if p.RequestSettings.Temperature > 0 {
		messageParams.Temperature = anthropic.Float(p.RequestSettings.Temperature)
	}

	// Add TopP if set
	if p.RequestSettings.TopP > 0 {
		messageParams.TopP = anthropic.Float(p.RequestSettings.TopP)
	}

	// Add TopK if set
	if p.RequestSettings.TopK > 0 {
		messageParams.TopK = anthropic.Int(p.RequestSettings.TopK)
	}

	// Add stop sequences if set
	if len(p.RequestSettings.Stop) > 0 {
		messageParams.StopSequences = p.RequestSettings.Stop
	}

	// Add tools if provided
	if len(tools) > 0 {
		messageParams.Tools = tools
	}

	resp, err := p.client.Messages.New(ctx, messageParams)
	if err != nil {
		return nil, fmt.Errorf("anthropic api error: %w", err)
	}

	response := &types.GenerateResponse{
		FinishReason: mapStopReason(resp.StopReason),
		Model:        string(resp.Model),
		Usage: types.Usage{
			PromptTokens:      int(resp.Usage.InputTokens),
			CompletionTokens:  int(resp.Usage.OutputTokens),
			TotalTokens:       int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
			CachedInputTokens: int(resp.Usage.CacheReadInputTokens),
		},
	}

	// Extract content and tool calls from response
	for _, block := range resp.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			response.Content += b.Text
		case anthropic.ToolUseBlock:
			var args map[string]any
			if err := json.Unmarshal(b.Input, &args); err != nil {
				args = make(map[string]any)
			}
			response.ToolCalls = append(response.ToolCalls, types.ToolCall{
				ID:        b.ID,
				Name:      b.Name,
				Arguments: args,
			})
		}
	}

	return response, nil
}

// Stream implements the Stream method of the LanguageModel interface
func (p *Provider) Stream(ctx context.Context, req provider.GenerateRequest) (<-chan types.StreamEvent, <-chan error) {
	eventChan := make(chan types.StreamEvent, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		messages := p.convertMessages(req.Messages)
		tools := p.convertTools(req.Tools)

		messageParams := anthropic.MessageNewParams{
			Model:     anthropic.Model(p.RequestSettings.Model),
			MaxTokens: p.RequestSettings.MaxTokens,
			Messages:  messages,
		}

		// Add system prompt if provided
		if req.System != "" {
			messageParams.System = []anthropic.TextBlockParam{{
				Text: req.System,
			}}
		}

		// Add temperature if set
		if p.RequestSettings.Temperature > 0 {
			messageParams.Temperature = anthropic.Float(p.RequestSettings.Temperature)
		}

		// Add TopP if set
		if p.RequestSettings.TopP > 0 {
			messageParams.TopP = anthropic.Float(p.RequestSettings.TopP)
		}

		// Add TopK if set
		if p.RequestSettings.TopK > 0 {
			messageParams.TopK = anthropic.Int(p.RequestSettings.TopK)
		}

		// Add stop sequences if set
		if len(p.RequestSettings.Stop) > 0 {
			messageParams.StopSequences = p.RequestSettings.Stop
		}

		// Add tools if provided
		if len(tools) > 0 {
			messageParams.Tools = tools
		}

		stream := p.client.Messages.NewStreaming(ctx, messageParams)

		// Track state for building complete response
		var fullText string
		var modelID string
		toolCallsMap := make(map[int]*toolCallBuilder)
		var totalUsage types.Usage
		streamStarted := false

		for stream.Next() {
			event := stream.Current()

			switch e := event.AsAny().(type) {
			case anthropic.MessageStartEvent:
				modelID = string(e.Message.Model)
				eventChan <- types.NewStreamStartEvent(modelID, e.Message.ID, "")
				streamStarted = true

				// Initialize usage from message start
				totalUsage = types.Usage{
					PromptTokens:      int(e.Message.Usage.InputTokens),
					CachedInputTokens: int(e.Message.Usage.CacheReadInputTokens),
				}

			case anthropic.ContentBlockStartEvent:
				switch block := e.ContentBlock.AsAny().(type) {
				case anthropic.ToolUseBlock:
					toolCallsMap[int(e.Index)] = &toolCallBuilder{
						id:        block.ID,
						name:      block.Name,
						arguments: "",
					}
					eventChan <- types.NewToolCallStartEvent(block.ID, block.Name, int(e.Index))
				}

			case anthropic.ContentBlockDeltaEvent:
				switch delta := e.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					fullText += delta.Text
					eventChan <- types.NewTextDeltaEvent(delta.Text, int(e.Index))
				case anthropic.InputJSONDelta:
					if builder, ok := toolCallsMap[int(e.Index)]; ok {
						builder.arguments += delta.PartialJSON
						eventChan <- types.NewToolCallDeltaEvent(builder.id, delta.PartialJSON, int(e.Index))
					}
				}

			case anthropic.ContentBlockStopEvent:
				// Check if this was a tool use block
				if builder, ok := toolCallsMap[int(e.Index)]; ok {
					var args map[string]any
					if builder.arguments != "" {
						json.Unmarshal([]byte(builder.arguments), &args)
					}
					toolCall := types.ToolCall{
						ID:        builder.id,
						Name:      builder.name,
						Arguments: args,
					}
					eventChan <- types.NewToolCallCompleteEvent(toolCall, int(e.Index))
				}

			case anthropic.MessageDeltaEvent:
				totalUsage.CompletionTokens = int(e.Usage.OutputTokens)
				totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
				eventChan <- types.NewFinishReasonEvent(mapStopReason(e.Delta.StopReason))
				eventChan <- types.NewUsageEvent(totalUsage)

			case anthropic.MessageStopEvent:
				// Stream completed
			}
		}

		if err := stream.Err(); err != nil {
			errChan <- fmt.Errorf("stream error: %w", err)
			return
		}

		// Send completion events
		if fullText != "" {
			eventChan <- types.NewTextCompleteEvent(fullText)
		}

		// Send stream end event
		finishReason := "stop"
		if len(toolCallsMap) > 0 {
			finishReason = "tool_calls"
		}

		if !streamStarted {
			// If stream never started, still send end event
			eventChan <- types.NewStreamStartEvent(p.RequestSettings.Model, "", "")
		}

		eventChan <- types.NewStreamEndEvent(finishReason, totalUsage)
	}()

	return eventChan, errChan
}

// Helper type for building tool calls from deltas
type toolCallBuilder struct {
	id        string
	name      string
	arguments string
}

// ID returns the model identifier
func (p *Provider) ID() string {
	return fmt.Sprintf("anthropic:%s", p.RequestSettings.Model)
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

func (p *Provider) ProviderName() string {
	return "anthropic"
}

// convertMessages converts types.Message to Anthropic message format
func (p *Provider) convertMessages(messages []types.Message) []anthropic.MessageParam {
	var result []anthropic.MessageParam

	for _, msg := range messages {
		// Skip system messages - they're handled separately
		if msg.Role == types.RoleSystem {
			continue
		}

		var contentBlocks []anthropic.ContentBlockParamUnion

		// Determine the role - Anthropic only accepts "user" or "assistant"
		// Tool messages should be converted to user messages with tool_result blocks
		role := anthropic.MessageParamRole(msg.Role)
		if msg.Role == types.RoleTool {
			role = anthropic.MessageParamRoleUser
		}

		// Add text content if present (skip for tool role as it uses ToolResults)
		if msg.Content != "" && msg.Role != types.RoleTool {
			contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
				OfText: &anthropic.TextBlockParam{Text: msg.Content},
			})
		}

		// Add tool calls (for assistant messages)
		for _, tc := range msg.ToolCalls {
			input := tc.Arguments
			if input == nil {
				input = make(map[string]any)
			}
			contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
				OfToolUse: &anthropic.ToolUseBlockParam{
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				},
			})
		}

		// Add tool results (for user messages containing results or tool role messages)
		for _, tr := range msg.ToolResults {
			toolResultBlock := anthropic.ToolResultBlockParam{
				ToolUseID: tr.ToolCallID,
			}
			if tr.Content != "" {
				toolResultBlock.Content = []anthropic.ToolResultBlockParamContentUnion{{
					OfText: &anthropic.TextBlockParam{Text: tr.Content},
				}}
			}
			if tr.IsError {
				toolResultBlock.IsError = anthropic.Bool(true)
			}
			contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
				OfToolResult: &toolResultBlock,
			})
		}

		// Only add message if it has content
		if len(contentBlocks) > 0 {
			result = append(result, anthropic.MessageParam{
				Role:    role,
				Content: contentBlocks,
			})
		}
	}

	return result
}

// convertTools converts types.Tool to Anthropic tool format
func (p *Provider) convertTools(tools []types.Tool) []anthropic.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}

	result := make([]anthropic.ToolUnionParam, len(tools))
	for i, tool := range tools {
		// Convert parameters to input schema
		inputSchemaJSON, err := json.Marshal(tool.Parameters)
		if err != nil {
			continue
		}

		var inputSchema anthropic.ToolInputSchemaParam
		if err := json.Unmarshal(inputSchemaJSON, &inputSchema); err != nil {
			continue
		}

		result[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: inputSchema,
			},
		}
	}

	return result
}

// mapStopReason maps Anthropic stop reasons to standard format
func mapStopReason(reason anthropic.StopReason) string {
	switch reason {
	case anthropic.StopReasonEndTurn:
		return types.FinishReasonStop
	case anthropic.StopReasonMaxTokens:
		return types.FinishReasonLength
	case anthropic.StopReasonToolUse:
		return types.FinishReasonToolCalls
	case anthropic.StopReasonStopSequence:
		return types.FinishReasonStop
	default:
		return string(reason)
	}
}

// Model capability helpers
func isVisionModel(model string) bool {
	// All Claude 3+ models support vision
	return true
}

func getMaxContextTokens(model string) int {
	// All current Claude models support 200K context
	contextLimits := map[string]int{
		"claude-opus-4-5-20251101":   200000,
		"claude-haiku-4-5-20251001":  200000,
		"claude-sonnet-4-5-20250929": 200000,
		"claude-opus-4-1-20250805":   200000,
		"claude-opus-4-20250514":     200000,
		"claude-sonnet-4-20250514":   200000,
		"claude-3-7-sonnet-20250219": 200000,
		"claude-3-5-haiku-20241022":  200000,
		"claude-3-haiku-20240307":    200000,
		"claude-3-opus-20240229":     200000,
	}
	if limit, ok := contextLimits[model]; ok {
		return limit
	}
	return 200000 // default for Claude 3+
}

func getMaxOutputTokens(model string) int {
	outputLimits := map[string]int{
		"claude-opus-4-5-20251101":   32000,
		"claude-haiku-4-5-20251001":  8192,
		"claude-sonnet-4-5-20250929": 16384,
		"claude-opus-4-1-20250805":   32000,
		"claude-opus-4-20250514":     32000,
		"claude-sonnet-4-20250514":   16384,
		"claude-3-7-sonnet-20250219": 16384,
		"claude-3-5-haiku-20241022":  8192,
		"claude-3-haiku-20240307":    4096,
		"claude-3-opus-20240229":     4096,
	}
	if limit, ok := outputLimits[model]; ok {
		return limit
	}
	return 8192 // default
}

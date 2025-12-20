package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
)

// Provider implements the LanguageModel interface for Anthropic Claude
type Provider struct {
	client anthropic.Client
	model  string
	config Config
}

// Config holds Anthropic-specific configuration
type Config struct {
	APIKey      string
	Model       string
	BaseURL     string
	Temperature float32
	MaxTokens   int

	// Prompt caching options
	// CacheSystemPrompt automatically adds cache control to system prompts
	CacheSystemPrompt bool
	// CacheTools automatically adds cache control to tools
	CacheTools bool
	// CacheBreakpoints specifies message indices where cache breakpoints should be set
	// Useful for caching long conversation context
	CacheBreakpoints []int
}

// New creates a new Anthropic provider
func New(apiKey, model string) *Provider {
	return NewWithConfig(Config{
		APIKey: apiKey,
		Model:  model,
	})
}

// NewWithConfig creates a new Anthropic provider with custom configuration
func NewWithConfig(config Config) *Provider {
	opts := []option.RequestOption{option.WithAPIKey(config.APIKey)}

	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}

	client := anthropic.NewClient(opts...)

	return &Provider{
		client: client,
		model:  config.Model,
		config: config,
	}
}

// ID returns the model identifier
func (p *Provider) ID() string {
	return fmt.Sprintf("anthropic:%s", p.model)
}

// Capabilities returns the model's capabilities
func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsTools:     true,
		SupportsStreaming: true,
		SupportsVision:    true,   // All Claude 3+ models support vision
		MaxContextTokens:  200000, // All Claude models have 200k context
		MaxOutputTokens:   getMaxOutputTokens(p.model),
	}
}

// Generate implements the Generate method of the LanguageModel interface
func (p *Provider) Generate(ctx context.Context, req provider.GenerateRequest) (*types.GenerateResponse, error) {
	// Convert messages and extract system prompt
	messages, systemPrompt, cacheControlUsed := p.convertMessages(req.Messages, req.System)
	tools := p.convertTools(req.Tools, cacheControlUsed)

	// Build the message request
	msgReq := anthropic.MessageNewParams{
		Model:    anthropic.Model(p.model),
		Messages: messages,
	}

	// Set system prompt if present
	if len(systemPrompt) > 0 {
		msgReq.System = systemPrompt
	}

	// Apply parameters
	if req.MaxTokens > 0 {
		msgReq.MaxTokens = int64(req.MaxTokens)
	} else if p.config.MaxTokens > 0 {
		msgReq.MaxTokens = int64(p.config.MaxTokens)
	} else {
		// Anthropic requires max_tokens, set a reasonable default
		msgReq.MaxTokens = int64(4096)
	}

	if req.Temperature > 0 {
		msgReq.Temperature = anthropic.Float(float64(req.Temperature))
	} else if p.config.Temperature > 0 {
		msgReq.Temperature = anthropic.Float(float64(p.config.Temperature))
	}

	if req.TopP > 0 {
		msgReq.TopP = anthropic.Float(float64(req.TopP))
	}

	if req.TopK > 0 {
		msgReq.TopK = anthropic.Int(int64(req.TopK))
	}

	if len(req.Stop) > 0 {
		msgReq.StopSequences = req.Stop
	}

	if len(tools) > 0 {
		msgReq.Tools = tools
	}

	// Create the message
	resp, err := p.client.Messages.New(ctx, msgReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic api error: %w", err)
	}

	// Parse response
	response := &types.GenerateResponse{
		Model:        string(resp.Model),
		FinishReason: string(resp.StopReason),
		Usage: types.Usage{
			PromptTokens:      int(resp.Usage.InputTokens),
			CompletionTokens:  int(resp.Usage.OutputTokens),
			TotalTokens:       int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
			CachedInputTokens: int(resp.Usage.CacheReadInputTokens),
		},
	}

	// Extract content and tool calls from content blocks
	var textContent strings.Builder
	var toolCalls []types.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textContent.WriteString(block.Text)
		case "tool_use":
			args := make(map[string]interface{})
			if len(block.Input) > 0 {
				// Input is json.RawMessage, unmarshal it
				json.Unmarshal(block.Input, &args)
			}
			toolCalls = append(toolCalls, types.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	response.Content = textContent.String()
	response.ToolCalls = toolCalls

	return response, nil
}

// Stream implements the Stream method of the LanguageModel interface
func (p *Provider) Stream(ctx context.Context, req provider.GenerateRequest) (<-chan types.StreamEvent, <-chan error) {
	eventChan := make(chan types.StreamEvent, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		// Convert messages and extract system prompt
		messages, systemPrompt, cacheControlUsed := p.convertMessages(req.Messages, req.System)
		tools := p.convertTools(req.Tools, cacheControlUsed)

		// Build the message request
		msgReq := anthropic.MessageNewParams{
			Model:    anthropic.Model(p.model),
			Messages: messages,
		}

		// Set system prompt if present
		if len(systemPrompt) > 0 {
			msgReq.System = systemPrompt
		}

		// Apply parameters
		if req.MaxTokens > 0 {
			msgReq.MaxTokens = int64(req.MaxTokens)
		} else if p.config.MaxTokens > 0 {
			msgReq.MaxTokens = int64(p.config.MaxTokens)
		} else {
			msgReq.MaxTokens = int64(4096)
		}

		if req.Temperature > 0 {
			msgReq.Temperature = anthropic.Float(float64(req.Temperature))
		} else if p.config.Temperature > 0 {
			msgReq.Temperature = anthropic.Float(float64(p.config.Temperature))
		}

		if req.TopP > 0 {
			msgReq.TopP = anthropic.Float(float64(req.TopP))
		}

		if req.TopK > 0 {
			msgReq.TopK = anthropic.Int(int64(req.TopK))
		}

		if len(req.Stop) > 0 {
			msgReq.StopSequences = req.Stop
		}

		if len(tools) > 0 {
			msgReq.Tools = tools
		}

		// Create streaming request
		stream := p.client.Messages.NewStreaming(ctx, msgReq)

		// Track state for building complete tool calls
		toolCallBuilders := make(map[int]*toolCallBuilder)
		var fullText string
		var totalUsage types.Usage
		var modelID string
		var messageID string
		streamStarted := false

		// Process stream events
		for stream.Next() {
			event := stream.Current()

			switch event.Type {
			case "message_start":
				messageStart := event.Message
				if !streamStarted {
					modelID = string(messageStart.Model)
					messageID = messageStart.ID
					eventChan <- types.NewStreamStartEvent(modelID, messageID, "")
					streamStarted = true
				}

				// Initial usage from message start
				totalUsage.PromptTokens = int(messageStart.Usage.InputTokens)
				totalUsage.CachedInputTokens = int(messageStart.Usage.CacheReadInputTokens)
				eventChan <- types.NewUsageEvent(totalUsage)

			case "content_block_start":
				block := event.ContentBlock
				index := int(event.Index)

				if block.Type == "tool_use" {
					// Start a new tool call
					toolCallBuilders[index] = &toolCallBuilder{
						id:        block.ID,
						name:      block.Name,
						arguments: "",
					}
					eventChan <- types.NewToolCallStartEvent(block.ID, block.Name, index)
				}

			case "content_block_delta":
				delta := event.Delta
				index := int(event.Index)

				switch delta.Type {
				case "text_delta":
					fullText += delta.Text
					eventChan <- types.NewTextDeltaEvent(delta.Text, index)

				case "input_json_delta":
					if builder, exists := toolCallBuilders[index]; exists {
						builder.arguments += delta.PartialJSON
						eventChan <- types.NewToolCallDeltaEvent(
							builder.id,
							delta.PartialJSON,
							index,
						)
					}
				}

			case "content_block_stop":
				index := int(event.Index)
				if builder, exists := toolCallBuilders[index]; exists {
					// Complete the tool call
					args := make(map[string]interface{})
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

			case "message_delta":
				// Update usage with output tokens
				totalUsage.CompletionTokens = int(event.Usage.OutputTokens)
				totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
				eventChan <- types.NewUsageEvent(totalUsage)

				// Send finish reason
				if event.Delta.StopReason != "" {
					eventChan <- types.NewFinishReasonEvent(string(event.Delta.StopReason))
				}

			case "message_stop":
				// Stream ended
				if fullText != "" {
					eventChan <- types.NewTextCompleteEvent(fullText)
				}
				eventChan <- types.NewStreamEndEvent("stop", totalUsage)
			}
		}

		// Check for streaming errors
		if err := stream.Err(); err != nil {
			errChan <- fmt.Errorf("anthropic stream error: %w", err)
			return
		}
	}()

	return eventChan, errChan
}

// Helper type for building tool calls from deltas
type toolCallBuilder struct {
	id        string
	name      string
	arguments string
}

// convertMessages converts SDK messages to Anthropic format and extracts system messages
// Anthropic allows a maximum of 4 cache control breakpoints per request
func (p *Provider) convertMessages(messages []types.Message, systemPrompt string) ([]anthropic.MessageParam, []anthropic.TextBlockParam, int) {
	const maxCacheControlBlocks = 4
	cacheControlCount := 0

	result := make([]anthropic.MessageParam, 0, len(messages))
	var system []anthropic.TextBlockParam

	// Extract system messages and convert to system prompt
	var systemTexts []string
	var shouldCacheSystem bool
	if systemPrompt != "" {
		systemTexts = append(systemTexts, systemPrompt)
	}

	// Check if any system message has cache control in metadata
	for _, msg := range messages {
		if msg.Role == types.RoleSystem {
			systemTexts = append(systemTexts, msg.Content)
			// Check metadata for cache control hint
			if cacheControl, ok := msg.Metadata["cache_control"]; ok {
				if cc, ok := cacheControl.(map[string]interface{}); ok {
					if ccType, ok := cc["type"].(string); ok && ccType == "ephemeral" {
						shouldCacheSystem = true
					}
				}
			}
		}
	}

	// Apply automatic caching from config
	if p.config.CacheSystemPrompt {
		shouldCacheSystem = true
	}

	// Build cache breakpoints set for fast lookup
	cacheBreakpoints := make(map[int]bool)
	for _, idx := range p.config.CacheBreakpoints {
		cacheBreakpoints[idx] = true
	}

	messageIndex := 0
	for _, msg := range messages {
		if msg.Role == types.RoleSystem {
			continue // Already handled above
		}

		// Build content blocks for this message
		var contentBlocks []anthropic.ContentBlockParamUnion

		// Add text content if present
		if msg.Content != "" {
			// Check if this message index is a cache breakpoint
			// or has cache control in metadata
			shouldCacheMessage := cacheBreakpoints[messageIndex]
			if !shouldCacheMessage {
				if cacheControl, ok := msg.Metadata["cache_control"]; ok {
					if cc, ok := cacheControl.(map[string]interface{}); ok {
						if ccType, ok := cc["type"].(string); ok && ccType == "ephemeral" {
							shouldCacheMessage = true
						}
					}
				}
			}

			// Only add cache control if we haven't exceeded the limit
			if shouldCacheMessage && cacheControlCount < maxCacheControlBlocks {
				// Create text block with cache control
				textBlock := anthropic.TextBlockParam{
					Text: msg.Content,
					Type: "text",
					CacheControl: anthropic.CacheControlEphemeralParam{
						Type: "ephemeral",
					},
				}
				contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
					OfText: &textBlock,
				})
				cacheControlCount++
			} else {
				// Use the convenience constructor for simple text blocks
				contentBlocks = append(contentBlocks, anthropic.NewTextBlock(msg.Content))
			}
		}

		// Add tool calls as tool_use blocks (for assistant messages)
		if msg.Role == types.RoleAssistant && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				// Ensure arguments is a valid dictionary (non-nil map)
				var input map[string]any
				if tc.Arguments != nil {
					input = tc.Arguments
				} else {
					input = make(map[string]any)
				}
				contentBlocks = append(contentBlocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
			}
		}

		// Add tool results as tool_result blocks (for user or tool messages)
		if len(msg.ToolResults) > 0 {
			for _, tr := range msg.ToolResults {
				contentBlocks = append(contentBlocks, anthropic.NewToolResultBlock(
					tr.ToolCallID,
					tr.Content,
					tr.IsError,
				))
			}
		}

		// Map role (skip system as it's already handled)
		role := anthropic.MessageParamRole(msg.Role)

		// Tool role messages become user messages in Anthropic with tool_result blocks
		if msg.Role == types.RoleTool {
			role = anthropic.MessageParamRoleUser
		}

		// Only add message if it has content blocks
		if len(contentBlocks) > 0 {
			result = append(result, anthropic.MessageParam{
				Role:    role,
				Content: contentBlocks,
			})
			messageIndex++
		}
	}

	// Convert system texts to TextBlockParam with cache control if needed
	if len(systemTexts) > 0 {
		combinedSystem := strings.Join(systemTexts, "\n\n")
		textBlock := anthropic.TextBlockParam{
			Text: combinedSystem,
			Type: "text",
		}

		// Only add cache control if we haven't exceeded the limit
		if shouldCacheSystem && cacheControlCount < maxCacheControlBlocks {
			textBlock.CacheControl = anthropic.CacheControlEphemeralParam{
				Type: "ephemeral",
			}
			cacheControlCount++
		}

		system = []anthropic.TextBlockParam{textBlock}
	}

	return result, system, cacheControlCount
}

// convertTools converts SDK tools to Anthropic format with optional cache control
// cacheControlUsed is the number of cache control blocks already used by messages/system
func (p *Provider) convertTools(tools []types.Tool, cacheControlUsed int) []anthropic.ToolUnionParam {
	const maxCacheControlBlocks = 4

	if len(tools) == 0 {
		return nil
	}

	result := make([]anthropic.ToolUnionParam, len(tools))
	for i, tool := range tools {
		// The tool.Parameters is already a complete JSON Schema object
		// Extract the properties and required fields from it
		inputSchema := anthropic.ToolInputSchemaParam{
			Type: "object",
		}

		// Extract properties if they exist
		if properties, ok := tool.Parameters["properties"]; ok {
			inputSchema.Properties = properties
		}

		// Extract required fields if they exist
		if required, ok := tool.Parameters["required"].([]interface{}); ok {
			reqStrings := make([]string, len(required))
			for j, r := range required {
				if s, ok := r.(string); ok {
					reqStrings[j] = s
				}
			}
			inputSchema.Required = reqStrings
		}

		// Store any additional fields that might be present in the schema
		inputSchema.ExtraFields = make(map[string]any)
		for key, value := range tool.Parameters {
			if key != "type" && key != "properties" && key != "required" {
				inputSchema.ExtraFields[key] = value
			}
		}

		toolParam := anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
			InputSchema: inputSchema,
		}

		// Add cache control to the last tool if CacheTools is enabled
		// and we haven't exceeded the 4-block limit
		// This follows Anthropic's best practice of caching tools at the end
		if p.config.CacheTools && i == len(tools)-1 && cacheControlUsed < maxCacheControlBlocks {
			toolParam.CacheControl = anthropic.CacheControlEphemeralParam{
				Type: "ephemeral",
			}
		}

		result[i] = anthropic.ToolUnionParam{
			OfTool: &toolParam,
		}
	}
	return result
}

// getMaxOutputTokens returns the maximum output tokens for a model
func getMaxOutputTokens(model string) int {
	// Claude 3.5 Sonnet and newer models support 8192 output tokens
	if strings.Contains(model, "claude-3-5") ||
		strings.Contains(model, "claude-4") {
		return 8192
	}
	// Older Claude 3 models support 4096 output tokens
	return 4096
}

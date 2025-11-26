package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"google.golang.org/genai"
)

// Provider implements the LanguageModel interface for Google Gemini
type Provider struct {
	client *genai.Client
	model  string
	config Config
}

// Config holds Gemini-specific configuration
type Config struct {
	APIKey      string
	Model       string
	Temperature float32
	MaxTokens   int
	TopK        int
	TopP        float32
}

// New creates a new Gemini provider with API key
func New(apiKey, model string) (*Provider, error) {
	return NewWithConfig(Config{
		APIKey: apiKey,
		Model:  model,
	})
}

// NewWithConfig creates a new Gemini provider with custom configuration
func NewWithConfig(config Config) (*Provider, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  config.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	return &Provider{
		client: client,
		model:  config.Model,
		config: config,
	}, nil
}

// Note: genai.Client doesn't have a Close method, connections are managed automatically

// ID returns the model identifier
func (p *Provider) ID() string {
	return fmt.Sprintf("gemini:%s", p.model)
}

// Capabilities returns the model's capabilities
func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsTools:     true,
		SupportsStreaming: true,
		SupportsVision:    isVisionModel(p.model),
		MaxContextTokens:  getMaxContextTokens(p.model),
		MaxOutputTokens:   getMaxOutputTokens(p.model),
	}
}

// Generate implements the Generate method of the LanguageModel interface
func (p *Provider) Generate(ctx context.Context, req provider.GenerateRequest) (*types.GenerateResponse, error) {
	// Convert messages to Gemini format
	contents, systemInstruction, err := p.convertMessages(req.Messages, req.System)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	// Build generation config
	genConfig := &genai.GenerateContentConfig{}

	// Apply parameters
	if req.Temperature > 0 {
		genConfig.Temperature = &req.Temperature
	} else if p.config.Temperature > 0 {
		genConfig.Temperature = &p.config.Temperature
	}

	if req.MaxTokens > 0 {
		genConfig.MaxOutputTokens = int32(req.MaxTokens)
	} else if p.config.MaxTokens > 0 {
		genConfig.MaxOutputTokens = int32(p.config.MaxTokens)
	}

	if req.TopP > 0 {
		genConfig.TopP = &req.TopP
	} else if p.config.TopP > 0 {
		genConfig.TopP = &p.config.TopP
	}

	if req.TopK > 0 {
		topK := float32(req.TopK)
		genConfig.TopK = &topK
	} else if p.config.TopK > 0 {
		topK := float32(p.config.TopK)
		genConfig.TopK = &topK
	}

	if len(req.Stop) > 0 {
		genConfig.StopSequences = req.Stop
	}

	// Set system instruction if present
	if systemInstruction != nil {
		genConfig.SystemInstruction = systemInstruction
	}

	// Convert and add tools if present
	if len(req.Tools) > 0 {
		tools, err := p.convertTools(req.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tools: %w", err)
		}
		genConfig.Tools = tools
	}

	// Generate content
	resp, err := p.client.Models.GenerateContent(ctx, p.model, contents, genConfig)
	if err != nil {
		return nil, fmt.Errorf("gemini api error: %w", err)
	}

	// Parse response
	return p.parseResponse(resp)
}

// Stream implements the Stream method of the LanguageModel interface
func (p *Provider) Stream(ctx context.Context, req provider.GenerateRequest) (<-chan types.StreamEvent, <-chan error) {
	eventChan := make(chan types.StreamEvent, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		// Convert messages to Gemini format
		contents, systemInstruction, err := p.convertMessages(req.Messages, req.System)
		if err != nil {
			errChan <- fmt.Errorf("failed to convert messages: %w", err)
			return
		}

		// Build generation config
		genConfig := &genai.GenerateContentConfig{}

		// Apply parameters
		if req.Temperature > 0 {
			genConfig.Temperature = &req.Temperature
		} else if p.config.Temperature > 0 {
			genConfig.Temperature = &p.config.Temperature
		}

		if req.MaxTokens > 0 {
			genConfig.MaxOutputTokens = int32(req.MaxTokens)
		} else if p.config.MaxTokens > 0 {
			genConfig.MaxOutputTokens = int32(p.config.MaxTokens)
		}

		if req.TopP > 0 {
			genConfig.TopP = &req.TopP
		} else if p.config.TopP > 0 {
			genConfig.TopP = &p.config.TopP
		}

		if req.TopK > 0 {
			topK := float32(req.TopK)
			genConfig.TopK = &topK
		} else if p.config.TopK > 0 {
			topK := float32(p.config.TopK)
			genConfig.TopK = &topK
		}

		if len(req.Stop) > 0 {
			genConfig.StopSequences = req.Stop
		}

		// Set system instruction if present
		if systemInstruction != nil {
			genConfig.SystemInstruction = systemInstruction
		}

		// Convert and add tools if present
		if len(req.Tools) > 0 {
			tools, err := p.convertTools(req.Tools)
			if err != nil {
				errChan <- fmt.Errorf("failed to convert tools: %w", err)
				return
			}
			genConfig.Tools = tools
		}

		// Create streaming iterator
		iter := p.client.Models.GenerateContentStream(ctx, p.model, contents, genConfig)

		// Track state for building complete responses
		toolCallBuilders := make(map[int]*toolCallBuilder)
		var fullText strings.Builder
		var totalUsage types.Usage
		streamStarted := false
		toolCallIndex := 0

		// Process streaming responses
		for resp, err := range iter {
			if err != nil {
				errChan <- fmt.Errorf("stream error: %w", err)
				return
			}

			// Send stream start event on first chunk
			if !streamStarted {
				eventChan <- types.NewStreamStartEvent(p.model, "", "")
				streamStarted = true
			}

			// Process candidates
			if len(resp.Candidates) > 0 {
				candidate := resp.Candidates[0]

				// Process content parts
				if candidate.Content != nil {
					for _, part := range candidate.Content.Parts {
						// Handle text parts
						if text := part.Text; text != "" {
							fullText.WriteString(text)
							eventChan <- types.NewTextDeltaEvent(text, 0)
						}

						// Handle function call parts
						if funcCall := part.FunctionCall; funcCall != nil {
							// Check if this is a new tool call
							if _, exists := toolCallBuilders[toolCallIndex]; !exists {
								// Generate an ID for the tool call
								id := fmt.Sprintf("call_%d", toolCallIndex)
								toolCallBuilders[toolCallIndex] = &toolCallBuilder{
									id:        id,
									name:      funcCall.Name,
									arguments: "",
								}
								eventChan <- types.NewToolCallStartEvent(id, funcCall.Name, toolCallIndex)
							}

							// Accumulate arguments
							if funcCall.Args != nil {
								argsJSON, _ := json.Marshal(funcCall.Args)
								toolCallBuilders[toolCallIndex].arguments += string(argsJSON)
								eventChan <- types.NewToolCallDeltaEvent(
									toolCallBuilders[toolCallIndex].id,
									string(argsJSON),
									toolCallIndex,
								)
							}

							toolCallIndex++
						}
					}
				}

				// Handle finish reason
				if candidate.FinishReason != genai.FinishReasonUnspecified {
					finishReason := convertFinishReason(candidate.FinishReason)
					eventChan <- types.NewFinishReasonEvent(finishReason)
				}
			}

			// Handle usage metadata
			if resp.UsageMetadata != nil {
				totalUsage = types.Usage{
					PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
					CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
					TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
				}
				eventChan <- types.NewUsageEvent(totalUsage)
			}
		}

		// Send completion events
		if fullText.Len() > 0 {
			eventChan <- types.NewTextCompleteEvent(fullText.String())
		}

		// Send complete tool calls
		for index, builder := range toolCallBuilders {
			var args map[string]interface{}
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

		// Send stream end event
		eventChan <- types.NewStreamEndEvent("stop", totalUsage)
	}()

	return eventChan, errChan
}

// Helper type for building tool calls from streaming chunks
type toolCallBuilder struct {
	id        string
	name      string
	arguments string
}

// convertMessages converts SDK messages to Gemini Content format
func (p *Provider) convertMessages(messages []types.Message, systemPrompt string) ([]*genai.Content, *genai.Content, error) {
	contents := make([]*genai.Content, 0, len(messages))
	var systemInstruction *genai.Content

	// Handle system prompt
	var systemTexts []string
	if systemPrompt != "" {
		systemTexts = append(systemTexts, systemPrompt)
	}

	// Extract system messages
	for _, msg := range messages {
		if msg.Role == types.RoleSystem {
			systemTexts = append(systemTexts, msg.Content)
		}
	}

	// Create system instruction if we have system texts
	if len(systemTexts) > 0 {
		combinedSystem := strings.Join(systemTexts, "\n\n")
		systemInstruction = &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(combinedSystem)},
		}
	}

	// Convert regular messages
	for _, msg := range messages {
		// Skip system messages (already handled)
		if msg.Role == types.RoleSystem {
			continue
		}

		// Map role to Gemini role
		role := convertRole(msg.Role)

		// Build parts for this message
		var parts []*genai.Part

		// Add text content if present
		if msg.Content != "" {
			parts = append(parts, genai.NewPartFromText(msg.Content))
		}

		// Add tool calls as function call parts (for assistant messages)
		if msg.Role == types.RoleAssistant && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				// Ensure arguments is valid
				args := tc.Arguments
				if args == nil {
					args = make(map[string]interface{})
				}
				parts = append(parts, genai.NewPartFromFunctionCall(tc.Name, args))
			}
		}

		// Add tool results as function response parts (for tool messages)
		if len(msg.ToolResults) > 0 {
			for _, tr := range msg.ToolResults {
				// Parse the content as response (assume JSON or plain text)
				var responseData map[string]any
				if err := json.Unmarshal([]byte(tr.Content), &responseData); err != nil {
					// If not JSON, treat as plain string
					responseData = map[string]any{"result": tr.Content}
				}

				// Create function response part
				parts = append(parts, genai.NewPartFromFunctionResponse(
					extractFunctionNameFromToolCallID(tr.ToolCallID, messages),
					responseData,
				))
			}
		}

		// Only add content if it has parts
		if len(parts) > 0 {
			content := &genai.Content{
				Role:  role,
				Parts: parts,
			}
			contents = append(contents, content)
		}
	}

	return contents, systemInstruction, nil
}

// convertTools converts SDK tools to Gemini function declarations
func (p *Provider) convertTools(tools []types.Tool) ([]*genai.Tool, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	functionDeclarations := make([]*genai.FunctionDeclaration, len(tools))
	for i, tool := range tools {
		// Create function declaration
		funcDecl := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
		}

		// Convert JSON schema to Gemini schema format
		if tool.Parameters != nil {
			schema := &genai.Schema{
				Type: genai.TypeObject,
			}

			// Extract properties
			if properties, ok := tool.Parameters["properties"].(map[string]interface{}); ok {
				schema.Properties = make(map[string]*genai.Schema)
				for propName, propValue := range properties {
					if propSchema, ok := propValue.(map[string]interface{}); ok {
						schema.Properties[propName] = convertJSONSchemaToGeminiSchema(propSchema)
					}
				}
			}

			// Extract required fields
			if required, ok := tool.Parameters["required"].([]interface{}); ok {
				schema.Required = make([]string, len(required))
				for j, r := range required {
					if s, ok := r.(string); ok {
						schema.Required[j] = s
					}
				}
			}

			funcDecl.Parameters = schema
		}

		functionDeclarations[i] = funcDecl
	}

	return []*genai.Tool{{FunctionDeclarations: functionDeclarations}}, nil
}

// convertJSONSchemaToGeminiSchema converts JSON schema property to Gemini schema
func convertJSONSchemaToGeminiSchema(jsonSchema map[string]interface{}) *genai.Schema {
	schema := &genai.Schema{}

	// Map type
	if typeStr, ok := jsonSchema["type"].(string); ok {
		switch typeStr {
		case "string":
			schema.Type = genai.TypeString
		case "number":
			schema.Type = genai.TypeNumber
		case "integer":
			schema.Type = genai.TypeInteger
		case "boolean":
			schema.Type = genai.TypeBoolean
		case "array":
			schema.Type = genai.TypeArray
		case "object":
			schema.Type = genai.TypeObject
		}
	}

	// Map description
	if desc, ok := jsonSchema["description"].(string); ok {
		schema.Description = desc
	}

	// Map enum
	if enum, ok := jsonSchema["enum"].([]interface{}); ok {
		schema.Enum = make([]string, len(enum))
		for i, e := range enum {
			if s, ok := e.(string); ok {
				schema.Enum[i] = s
			}
		}
	}

	// Map array items
	if items, ok := jsonSchema["items"].(map[string]interface{}); ok {
		schema.Items = convertJSONSchemaToGeminiSchema(items)
	}

	// Map nested properties
	if properties, ok := jsonSchema["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for propName, propValue := range properties {
			if propSchema, ok := propValue.(map[string]interface{}); ok {
				schema.Properties[propName] = convertJSONSchemaToGeminiSchema(propSchema)
			}
		}
	}

	return schema
}

// parseResponse converts Gemini response to SDK response
func (p *Provider) parseResponse(resp *genai.GenerateContentResponse) (*types.GenerateResponse, error) {
	if len(resp.Candidates) == 0 {
		return nil, types.ErrEmptyResponse
	}

	candidate := resp.Candidates[0]
	response := &types.GenerateResponse{
		Model: p.model,
	}

	// Handle usage metadata
	if resp.UsageMetadata != nil {
		response.Usage = types.Usage{
			PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
		}
	}

	// Handle finish reason
	if candidate.FinishReason != genai.FinishReasonUnspecified {
		response.FinishReason = convertFinishReason(candidate.FinishReason)
	}

	// Extract content and tool calls
	var textContent strings.Builder
	var toolCalls []types.ToolCall
	toolCallIndex := 0

	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			// Handle text parts
			if text := part.Text; text != "" {
				textContent.WriteString(text)
			}

			// Handle function call parts
			if funcCall := part.FunctionCall; funcCall != nil {
				args := make(map[string]interface{})
				if funcCall.Args != nil {
					args = funcCall.Args
				}
				toolCalls = append(toolCalls, types.ToolCall{
					ID:        fmt.Sprintf("call_%d", toolCallIndex),
					Name:      funcCall.Name,
					Arguments: args,
				})
				toolCallIndex++
			}
		}
	}

	response.Content = textContent.String()
	response.ToolCalls = toolCalls

	return response, nil
}

// Helper functions

// convertRole converts SDK role to Gemini role
func convertRole(role types.MessageRole) string {
	switch role {
	case types.RoleUser, types.RoleTool:
		return "user"
	case types.RoleAssistant:
		return "model"
	case types.RoleSystem:
		return "user" // System messages are handled separately as system instruction
	default:
		return "user"
	}
}

// convertFinishReason converts Gemini finish reason to SDK format
func convertFinishReason(reason genai.FinishReason) string {
	switch reason {
	case genai.FinishReasonStop:
		return "stop"
	case genai.FinishReasonMaxTokens:
		return "length"
	case genai.FinishReasonSafety:
		return "content_filter"
	case genai.FinishReasonRecitation:
		return "content_filter"
	case genai.FinishReasonOther:
		return "other"
	default:
		return "unknown"
	}
}

// extractFunctionNameFromToolCallID extracts the function name from previous tool call
// This is needed because Gemini function responses need the function name
func extractFunctionNameFromToolCallID(toolCallID string, messages []types.Message) string {
	// Search backwards through messages to find the tool call
	for i := len(messages) - 1; i >= 0; i-- {
		for _, tc := range messages[i].ToolCalls {
			if tc.ID == toolCallID {
				return tc.Name
			}
		}
	}
	// Fallback: try to extract from ID if it follows pattern
	// Default to "unknown" if not found
	return "unknown"
}

// Model capability helpers

func isVisionModel(model string) bool {
	// Gemini 1.5 and 2.0 models support vision
	return strings.Contains(model, "gemini-1.5") ||
		strings.Contains(model, "gemini-2.0") ||
		strings.Contains(model, "gemini-pro-vision")
}

func getMaxContextTokens(model string) int {
	contextLimits := map[string]int{
		"gemini-1.5-pro":    2097152, // 2M tokens
		"gemini-1.5-flash":  1048576, // 1M tokens
		"gemini-2.0-flash":  1048576, // 1M tokens
		"gemini-1.0-pro":    32768,   // 32K tokens
		"gemini-pro":        32768,   // 32K tokens
		"gemini-pro-vision": 16384,   // 16K tokens
	}

	// Check for exact match
	if limit, ok := contextLimits[model]; ok {
		return limit
	}

	// Check for partial matches
	if strings.Contains(model, "gemini-1.5-pro") {
		return 2097152
	}
	if strings.Contains(model, "gemini-1.5-flash") || strings.Contains(model, "gemini-2.0-flash") {
		return 1048576
	}

	// Default to 32K for unknown models
	return 32768
}

func getMaxOutputTokens(model string) int {
	// Most Gemini models support 8192 output tokens
	outputLimits := map[string]int{
		"gemini-1.5-pro":    8192,
		"gemini-1.5-flash":  8192,
		"gemini-2.0-flash":  8192,
		"gemini-1.0-pro":    2048,
		"gemini-pro":        2048,
		"gemini-pro-vision": 4096,
	}

	// Check for exact match
	if limit, ok := outputLimits[model]; ok {
		return limit
	}

	// Check for partial matches
	if strings.Contains(model, "gemini-1.5") || strings.Contains(model, "gemini-2.0") {
		return 8192
	}

	// Default to 8192 for unknown models
	return 8192
}

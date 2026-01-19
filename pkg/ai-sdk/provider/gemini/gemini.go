package gemini

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

// Provider implements the LanguageModel interface for Google Gemini
type Provider struct {
	client *genai.Client
	apiKey string

	RequestSettings RequestSettings
}

type RequestSettings struct {
	Model           string
	MaxOutputTokens int32
	Temperature     float32
	TopP            float32
	TopK            float32
	StopSequences   []string
}

// New creates a new Gemini provider
func New(ctx context.Context, apiKey, model string) (*Provider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	return &Provider{
		client: client,
		apiKey: apiKey,
		RequestSettings: RequestSettings{
			Model:           model,
			MaxOutputTokens: 4096, // Default max tokens for Gemini
		},
	}, nil
}

func (p *Provider) SetRequestSettings(settings RequestSettings) {
	p.RequestSettings = settings
}

// Generate implements the Generate method of the LanguageModel interface
func (p *Provider) Generate(ctx context.Context, req provider.GenerateRequest) (*types.GenerateResponse, error) {
	// Set generation config
	config := &genai.GenerateContentConfig{
		MaxOutputTokens: p.RequestSettings.MaxOutputTokens,
	}

	if p.RequestSettings.Temperature > 0 {
		config.Temperature = genai.Ptr(p.RequestSettings.Temperature)
	}
	if p.RequestSettings.TopP > 0 {
		config.TopP = genai.Ptr(p.RequestSettings.TopP)
	}
	if p.RequestSettings.TopK > 0 {
		config.TopK = genai.Ptr(p.RequestSettings.TopK)
	}
	if len(p.RequestSettings.StopSequences) > 0 {
		config.StopSequences = p.RequestSettings.StopSequences
	}

	// Set system instruction if provided
	if req.System != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(req.System)},
		}
	}

	// Convert tools if provided
	tools := p.convertTools(req.Tools)
	if len(tools) > 0 {
		config.Tools = tools
	}

	// Convert messages
	contents := p.convertMessages(req.Messages)

	resp, err := p.client.Models.GenerateContent(ctx, p.RequestSettings.Model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("gemini api error: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini returned no candidates")
	}

	candidate := resp.Candidates[0]

	response := &types.GenerateResponse{
		FinishReason: mapFinishReason(candidate.FinishReason),
		Model:        p.RequestSettings.Model,
		Usage:        types.Usage{},
	}

	// Extract usage if available
	if resp.UsageMetadata != nil {
		response.Usage = types.Usage{
			PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
		}
		if resp.UsageMetadata.CachedContentTokenCount > 0 {
			response.Usage.CachedInputTokens = int(resp.UsageMetadata.CachedContentTokenCount)
		}
	}

	// Extract content and tool calls from response
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				response.Content += part.Text
			}
			if part.FunctionCall != nil {
				toolCall := types.ToolCall{
					ID:        uuid.New().String(), // Gemini doesn't provide IDs, generate one
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Args,
				}
				// Store ThoughtSignature in metadata (required for Gemini 3 models)
				if len(part.ThoughtSignature) > 0 {
					toolCall.Metadata = map[string]any{
						"thought_signature": part.ThoughtSignature,
					}
				}
				response.ToolCalls = append(response.ToolCalls, toolCall)
			}
		}
	}

	return response, nil
}

func (p *Provider) Stream(ctx context.Context, req provider.GenerateRequest) (*provider.ProviderStream, error) {
	config := &genai.GenerateContentConfig{
		MaxOutputTokens: p.RequestSettings.MaxOutputTokens,
	}

	if p.RequestSettings.Temperature > 0 {
		config.Temperature = genai.Ptr(p.RequestSettings.Temperature)
	}
	if p.RequestSettings.TopP > 0 {
		config.TopP = genai.Ptr(p.RequestSettings.TopP)
	}
	if p.RequestSettings.TopK > 0 {
		config.TopK = genai.Ptr(p.RequestSettings.TopK)
	}
	if len(p.RequestSettings.StopSequences) > 0 {
		config.StopSequences = p.RequestSettings.StopSequences
	}

	if req.System != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(req.System)},
		}
	}

	tools := p.convertTools(req.Tools)
	if len(tools) > 0 {
		config.Tools = tools
	}

	contents := p.convertMessages(req.Messages)

	eventChan := make(chan types.StreamEvent, 100)
	ps := provider.NewProviderStream(eventChan)

	go func() {
		defer close(eventChan)

		var fullText string
		var totalUsage types.Usage
		streamStarted := false
		toolCallIndex := 0
		var toolCalls []types.ToolCall
		var streamErr error

		for resp, err := range p.client.Models.GenerateContentStream(ctx, p.RequestSettings.Model, contents, config) {
			if err != nil {
				streamErr = fmt.Errorf("stream error: %w", err)
				break
			}

			if !streamStarted {
				eventChan <- types.NewStreamStartEvent(p.RequestSettings.Model, uuid.New().String(), "")
				streamStarted = true
			}

			if resp.UsageMetadata != nil {
				totalUsage = types.Usage{
					PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
					CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
					TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
				}
				if resp.UsageMetadata.CachedContentTokenCount > 0 {
					totalUsage.CachedInputTokens = int(resp.UsageMetadata.CachedContentTokenCount)
				}
			}

			for _, candidate := range resp.Candidates {
				if candidate.Content == nil {
					continue
				}

				for _, part := range candidate.Content.Parts {
					if part.Text != "" {
						fullText += part.Text
						eventChan <- types.NewTextDeltaEvent(part.Text, 0)
					}

					if part.FunctionCall != nil {
						toolCall := types.ToolCall{
							ID:        uuid.New().String(),
							Name:      part.FunctionCall.Name,
							Arguments: part.FunctionCall.Args,
						}
						if len(part.ThoughtSignature) > 0 {
							toolCall.Metadata = map[string]any{
								"thought_signature": part.ThoughtSignature,
							}
						}
						toolCalls = append(toolCalls, toolCall)

						eventChan <- types.NewToolCallStartEvent(toolCall.ID, toolCall.Name, toolCallIndex)
						eventChan <- types.NewToolCallCompleteEvent(toolCall, toolCallIndex)
						toolCallIndex++
					}
				}
			}
		}

		if !streamStarted {
			eventChan <- types.NewStreamStartEvent(p.RequestSettings.Model, "", "")
		}

		if streamErr != nil {
			ps.SetError(streamErr)
		}

		if fullText != "" {
			eventChan <- types.NewTextCompleteEvent(fullText)
		}

		eventChan <- types.NewUsageEvent(totalUsage)

		finishReason := types.FinishReasonStop
		if streamErr != nil {
			finishReason = types.FinishReasonError
		} else if len(toolCalls) > 0 {
			finishReason = types.FinishReasonToolCalls
		}

		eventChan <- types.NewFinishReasonEvent(finishReason)
		eventChan <- types.NewStreamEndEvent(finishReason, totalUsage)
	}()

	return ps, nil
}

// ID returns the model identifier
func (p *Provider) ID() string {
	return fmt.Sprintf("gemini:%s", p.RequestSettings.Model)
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
	return "gemini"
}

// convertMessages converts types.Message to Gemini content format
func (p *Provider) convertMessages(messages []types.Message) []*genai.Content {
	var result []*genai.Content

	for _, msg := range messages {
		// Skip system messages - they're handled separately via SystemInstruction
		if msg.Role == types.RoleSystem {
			continue
		}

		var parts []*genai.Part

		// Determine the role - Gemini uses "user" or "model"
		role := "user"
		if msg.Role == types.RoleAssistant {
			role = "model"
		}

		// Add text content if present (skip for tool role as it uses FunctionResponse)
		if msg.Content != "" && msg.Role != types.RoleTool {
			parts = append(parts, genai.NewPartFromText(msg.Content))
		}

		// Add function calls (for model/assistant messages)
		for _, tc := range msg.ToolCalls {
			part := &genai.Part{
				FunctionCall: &genai.FunctionCall{
					Name: tc.Name,
					Args: tc.Arguments,
				},
			}
			// Extract ThoughtSignature from metadata (required for Gemini 3 models)
			if sig, ok := tc.Metadata["thought_signature"].([]byte); ok && len(sig) > 0 {
				part.ThoughtSignature = sig
			}
			parts = append(parts, part)
		}

		// Add function responses (for tool role messages)
		// We need to find the matching tool call to get its ThoughtSignature
		for _, tr := range msg.ToolResults {
			part := &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name: tr.ToolCallID,
					Response: map[string]any{
						"result":   tr.Content,
						"is_error": tr.IsError,
					},
				},
			}
			// Find matching tool call from previous messages to get ThoughtSignature
			for _, prevMsg := range messages {
				for _, tc := range prevMsg.ToolCalls {
					if tc.ID == tr.ToolCallID {
						if sig, ok := tc.Metadata["thought_signature"].([]byte); ok && len(sig) > 0 {
							part.ThoughtSignature = sig
						}
						break
					}
				}
			}
			parts = append(parts, part)
		}

		// Only add content if it has parts
		if len(parts) > 0 {
			result = append(result, &genai.Content{
				Role:  role,
				Parts: parts,
			})
		}
	}

	return result
}

// convertTools converts types.Tool to Gemini tool format
func (p *Provider) convertTools(tools []types.Tool) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	var functionDeclarations []*genai.FunctionDeclaration
	for _, tool := range tools {
		// Convert parameters map to genai.Schema
		schema := convertParametersToSchema(tool.Parameters)

		functionDeclarations = append(functionDeclarations, &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  schema,
		})
	}

	return []*genai.Tool{{
		FunctionDeclarations: functionDeclarations,
	}}
}

// convertParametersToSchema converts a JSON schema map to genai.Schema
func convertParametersToSchema(params map[string]any) *genai.Schema {
	if params == nil {
		return nil
	}

	schema := &genai.Schema{
		Type: genai.TypeObject,
	}

	// Extract type if present
	if typeVal, ok := params["type"].(string); ok {
		schema.Type = mapSchemaType(typeVal)
	}

	// Extract description if present
	if desc, ok := params["description"].(string); ok {
		schema.Description = desc
	}

	// Extract properties for object type
	if props, ok := params["properties"].(map[string]any); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for name, propVal := range props {
			if propMap, ok := propVal.(map[string]any); ok {
				schema.Properties[name] = convertParametersToSchema(propMap)
			}
		}
	}

	// Extract required fields
	if required, ok := params["required"].([]any); ok {
		for _, r := range required {
			if str, ok := r.(string); ok {
				schema.Required = append(schema.Required, str)
			}
		}
	}

	// Extract items for array type
	if items, ok := params["items"].(map[string]any); ok {
		schema.Items = convertParametersToSchema(items)
	}

	// Extract enum values
	if enumVals, ok := params["enum"].([]any); ok {
		for _, e := range enumVals {
			if str, ok := e.(string); ok {
				schema.Enum = append(schema.Enum, str)
			}
		}
	}

	return schema
}

// mapSchemaType converts JSON schema type to genai.Type
func mapSchemaType(t string) genai.Type {
	switch t {
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeUnspecified
	}
}

// mapFinishReason maps Gemini finish reasons to standard format
func mapFinishReason(reason genai.FinishReason) string {
	switch reason {
	case genai.FinishReasonStop:
		return types.FinishReasonStop
	case genai.FinishReasonMaxTokens:
		return types.FinishReasonLength
	case genai.FinishReasonSafety:
		return types.FinishReasonContentFilter
	case genai.FinishReasonRecitation:
		return types.FinishReasonContentFilter
	case genai.FinishReasonOther:
		return types.FinishReasonStop
	default:
		return types.FinishReasonStop
	}
}

// Model capability helpers
func isVisionModel(_ string) bool {
	// All Gemini models support vision
	return true
}

func getMaxContextTokens(model string) int {
	contextLimits := map[string]int{
		// Gemini 3 models
		"gemini-3-pro-preview":   1048576, // 1M tokens
		"gemini-3-flash-preview": 1048576,
		// Gemini 2.5 models
		"gemini-2.5-pro":        1048576,
		"gemini-2.5-flash":      1048576,
		"gemini-2.5-flash-lite": 1048576,
		// Gemini 2.0 models
		"gemini-2.0-flash":      1048576,
		"gemini-2.0-flash-lite": 1048576,
	}
	if limit, ok := contextLimits[model]; ok {
		return limit
	}
	return 1048576 // default 1M tokens
}

func getMaxOutputTokens(model string) int {
	outputLimits := map[string]int{
		// Gemini 3 models
		"gemini-3-pro-preview":   65536, // 64K tokens
		"gemini-3-flash-preview": 65536,
		// Gemini 2.5 models
		"gemini-2.5-pro":        65536,
		"gemini-2.5-flash":      65536,
		"gemini-2.5-flash-lite": 65536,
		// Gemini 2.0 models
		"gemini-2.0-flash":      8192,
		"gemini-2.0-flash-lite": 8192,
	}
	if limit, ok := outputLimits[model]; ok {
		return limit
	}
	return 8192 // default
}

package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/internal/domain"

	"github.com/rs/zerolog/log"
)

// ToolExecutor wraps an IntegrationExecutor with metadata for tool execution
type ToolExecutor struct {
	Executor        domain.IntegrationExecutor `json:"-"`
	IntegrationType domain.IntegrationType     `json:"integration_type"`
	NodeID          string                     `json:"node_id"`
	NodeName        string                     `json:"node_name"`
	CredentialID    string                     `json:"credential_id"`
	WorkspaceID     string                     `json:"workspace_id"`
	// AllowedActions specifies which actions from this integration should be available as tools
	// If empty, all actions are available (backward compatibility)
	AllowedActions []domain.IntegrationActionType `json:"allowed_actions,omitempty"`
	// WorkflowNode contains the full workflow node information including ProvidedByAgent configuration
	WorkflowNode *domain.WorkflowNode `json:"-"`
}

// ToolDefinition represents a tool that can be called by the LLM
type ToolDefinition struct {
	Name         string                       `json:"name"`
	Description  string                       `json:"description"`
	Parameters   map[string]interface{}       `json:"parameters"`
	ActionType   domain.IntegrationActionType `json:"action_type"`
	ToolExecutor *ToolExecutor                `json:"tool_executor"`
}

// ToolCallResult represents the result of a tool execution
type ToolCallResult struct {
	ToolName   string                       `json:"tool_name"`
	ActionType domain.IntegrationActionType `json:"action_type"`
	Success    bool                         `json:"success"`
	Result     interface{}                  `json:"result,omitempty"`
	Error      string                       `json:"error,omitempty"`
	Duration   time.Duration                `json:"duration"`
	StartTime  time.Time                    `json:"start_time"`
	EndTime    time.Time                    `json:"end_time"`
}

// ToolCallManager manages tool discovery, LLM tool definitions, and tool execution
type ToolCallManager interface {
	// DiscoverTools extracts all available tools from the provided executors
	DiscoverTools(ctx context.Context, toolExecutors []ToolExecutor) ([]ToolDefinition, error)

	// GetLLMTools converts tool definitions to LLM-compatible tool format
	GetLLMTools(toolDefinitions []ToolDefinition) []domain.Tool

	// ExecuteToolCall executes a tool call from LLM and returns the result
	ExecuteToolCall(ctx context.Context, toolCall domain.ToolCall, toolDefinitions []ToolDefinition) (*ToolCallResult, error)

	// FindTool finds a tool definition by name
	FindTool(toolName string, toolDefinitions []ToolDefinition) (*ToolDefinition, error)
}

// DefaultToolCallManager implements ToolCallManager
type DefaultToolCallManager struct {
	AgentNodeID                string
	executorIntegrationManager domain.ExecutorIntegrationManager
	parameterBinder            domain.IntegrationParameterBinder
	parameterResolver          *AIParameterResolver
	eventPublisher             domain.EventPublisher
}

type DefaultToolCallManagerDeps struct {
	AgentNodeID                string
	ExecutorIntegrationManager domain.ExecutorIntegrationManager
	ParameterBinder            domain.IntegrationParameterBinder
	EventPublisher             domain.EventPublisher
}

// NewDefaultToolCallManager creates a new DefaultToolCallManager
func NewDefaultToolCallManager(deps DefaultToolCallManagerDeps) ToolCallManager {
	return &DefaultToolCallManager{
		AgentNodeID:                deps.AgentNodeID,
		executorIntegrationManager: deps.ExecutorIntegrationManager,
		parameterBinder:            deps.ParameterBinder,
		parameterResolver:          NewAIParameterResolver(),
		eventPublisher:             deps.EventPublisher,
	}
}

// DiscoverTools extracts all available tools from the provided executors
func (m *DefaultToolCallManager) DiscoverTools(ctx context.Context, toolExecutors []ToolExecutor) ([]ToolDefinition, error) {
	var allTools []ToolDefinition

	for _, toolExec := range toolExecutors {
		log.Debug().
			Str("integration_type", string(toolExec.IntegrationType)).
			Str("node_id", toolExec.NodeID).
			Msg("Discovering tools for integration")

		// Get integration schema
		integration, err := m.executorIntegrationManager.GetIntegration(ctx, toolExec.IntegrationType)
		if err != nil {
			log.Error().
				Err(err).
				Str("integration_type", string(toolExec.IntegrationType)).
				Msg("Failed to get integration schema")
			continue // Skip this integration but continue with others
		}

		// Convert each action to a tool definition, filtering by allowed actions
		for _, action := range integration.Actions {
			// Check if this action is allowed (if AllowedActions is specified)
			if len(toolExec.AllowedActions) > 0 {
				allowed := false
				for _, allowedAction := range toolExec.AllowedActions {
					if action.ActionType == allowedAction {
						allowed = true
						break
					}
				}
				if !allowed {
					continue // Skip this action as it's not in the allowed list
				}
			}

			toolDef := m.convertActionToTool(action, &toolExec)
			allTools = append(allTools, toolDef)

			log.Debug().
				Str("tool_name", toolDef.Name).
				Str("action_type", string(toolDef.ActionType)).
				Msg("Discovered tool")
		}
	}

	log.Info().
		Int("total_tools", len(allTools)).
		Msg("Tool discovery completed")

	return allTools, nil
}

// convertActionToTool converts an IntegrationAction to a ToolDefinition
func (m *DefaultToolCallManager) convertActionToTool(action domain.IntegrationAction, toolExec *ToolExecutor) ToolDefinition {
	// Create a unique tool name combining integration and action
	toolName := fmt.Sprintf("%s_%s", strings.ToLower(string(toolExec.IntegrationType)), strings.ToLower(string(action.ActionType)))

	// Build parameters schema for LLM
	parameters := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
		"required":   []string{},
	}

	properties := parameters["properties"].(map[string]interface{})
	required := []string{}

	// Convert NodeProperty to JSON Schema format
	for _, prop := range action.Properties {
		propSchema := m.convertNodePropertyToSchema(prop)
		properties[prop.Key] = propSchema

		if prop.Required {
			required = append(required, prop.Key)
		}
	}

	parameters["required"] = required

	return ToolDefinition{
		Name:         toolName,
		Description:  fmt.Sprintf("%s: %s", action.Name, action.Description),
		Parameters:   parameters,
		ActionType:   action.ActionType,
		ToolExecutor: toolExec,
	}
}

// convertNodePropertyToSchema converts a NodeProperty to JSON Schema format
func (m *DefaultToolCallManager) convertNodePropertyToSchema(prop domain.NodeProperty) map[string]interface{} {
	schema := map[string]interface{}{
		"type": m.mapNodePropertyType(prop.Type),
	}

	if prop.Description != "" {
		schema["description"] = prop.Description
	}

	// Note: DefaultValue field doesn't exist in NodeProperty
	// Default values would be handled by the UI layer

	// Handle select/multiselect options
	if len(prop.Options) > 0 {
		var enumValues []interface{}
		for _, option := range prop.Options {
			enumValues = append(enumValues, option.Value)
		}
		schema["enum"] = enumValues
	}

	return schema
}

// mapNodePropertyType maps NodeProperty types to JSON Schema types
func (m *DefaultToolCallManager) mapNodePropertyType(propType domain.NodePropertyType) string {
	switch propType {
	case domain.NodePropertyType_String, domain.NodePropertyType_Text, domain.NodePropertyType_TagInput:
		return "string"
	case domain.NodePropertyType_Integer:
		return "integer"
	case domain.NodePropertyType_Number:
		return "number"
	case domain.NodePropertyType_Float:
		return "number"
	case domain.NodePropertyType_Boolean:
		return "boolean"
	case domain.NodePropertyType_Array:
		return "array"
	case domain.NodePropertyType_Object, domain.NodePropertyType_Map:
		return "object"
	case domain.NodePropertyType_Date:
		return "string" // ISO date string
	default:
		return "string" // Default to string for unknown types
	}
}

// GetLLMTools converts tool definitions to LLM-compatible tool format
func (m *DefaultToolCallManager) GetLLMTools(toolDefinitions []ToolDefinition) []domain.Tool {
	var llmTools []domain.Tool

	for _, toolDef := range toolDefinitions {
		llmTool := domain.Tool{
			Name:        toolDef.Name,
			Description: toolDef.Description,
			Parameters:  toolDef.Parameters,
		}
		llmTools = append(llmTools, llmTool)
	}

	return llmTools
}

// FindTool finds a tool definition by name
func (m *DefaultToolCallManager) FindTool(toolName string, toolDefinitions []ToolDefinition) (*ToolDefinition, error) {
	// Strip "functions." prefix if present (OpenAI function calling format)
	actualToolName := toolName
	if strings.HasPrefix(toolName, "functions.") {
		actualToolName = strings.TrimPrefix(toolName, "functions.")
	}

	for _, toolDef := range toolDefinitions {
		if toolDef.Name == actualToolName {
			return &toolDef, nil
		}
	}
	return nil, fmt.Errorf("tool not found: %s", toolName)
}

// ExecuteToolCall executes a tool call from LLM and returns the result
func (m *DefaultToolCallManager) ExecuteToolCall(ctx context.Context, toolCall domain.ToolCall, toolDefinitions []ToolDefinition) (*ToolCallResult, error) {
	startTime := time.Now()

	result := &ToolCallResult{
		ToolName:  toolCall.Name,
		StartTime: startTime,
	}

	// Find the tool definition
	toolDef, err := m.FindTool(toolCall.Name, toolDefinitions)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	result.ActionType = toolDef.ActionType
	toolNodeID := toolDef.ToolExecutor.NodeID

	// Get workflow execution context for event publishing
	workflowCtx, hasWorkflowCtx := domain.GetWorkflowExecutionContext(ctx)

	// Publish node execution started event
	if hasWorkflowCtx && workflowCtx.EnableEvents {
		err = m.publishToolExecutionStartedEvent(ctx, workflowCtx, toolNodeID)
		if err != nil {
			log.Error().Err(err).Str("tool_name", toolCall.Name).Msg("Failed to publish tool execution started event")
		}
	}

	// Resolve parameters using AI vs preset values
	resolvedParams, err := m.resolveToolParameters(toolCall, toolDef)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Parameter resolution failed: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Prepare integration input with resolved parameters
	integrationInput, err := m.createIntegrationInputWithResolvedParams(toolCall, toolDef, resolvedParams)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Execute the tool
	log.Debug().
		Str("tool_name", toolCall.Name).
		Str("action_type", string(toolDef.ActionType)).
		Interface("arguments", toolCall.Arguments).
		Msg("Executing tool call")

	output, err := toolDef.ToolExecutor.Executor.Execute(ctx, integrationInput)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Success = false
		result.Error = err.Error()

		// Publish node failed event
		if hasWorkflowCtx && workflowCtx.EnableEvents {
			publishErr := m.publishToolExecutionFailedEvent(ctx, workflowCtx, toolNodeID, toolCall, resolvedParams, err)
			if publishErr != nil {
				log.Error().Err(publishErr).Str("tool_name", toolCall.Name).Msg("Failed to publish tool execution failed event")
			}
		}

		log.Error().
			Err(err).
			Str("tool_name", toolCall.Name).
			Dur("duration", result.Duration).
			Msg("Tool execution failed")

		return result, err
	}

	result.Success = true
	result.Result = m.formatToolResult(output)

	// Publish node execution completed event
	if hasWorkflowCtx && workflowCtx.EnableEvents {
		err = m.publishToolExecutionCompletedEvent(ctx, workflowCtx, toolNodeID, resolvedParams, output)
		if err != nil {
			log.Error().Err(err).Str("tool_name", toolCall.Name).Msg("Failed to publish tool execution completed event")
		}
	}

	log.Debug().
		Str("tool_name", toolCall.Name).
		Dur("duration", result.Duration).
		Msg("Tool execution completed successfully")

	return result, nil
}

// resolveToolParameters resolves tool parameters using AI vs preset values
func (m *DefaultToolCallManager) resolveToolParameters(toolCall domain.ToolCall, toolDef *ToolDefinition) (*ParameterResolutionResult, error) {
	if toolDef.ToolExecutor.WorkflowNode == nil {
		// Fallback to simple parameter passing if no workflow node metadata
		log.Warn().
			Str("tool_name", toolCall.Name).
			Msg("No workflow node metadata available - using simple parameter passing")

		return &ParameterResolutionResult{
			ResolvedSettings: toolCall.Arguments,
			ResolutionLog:    []ParameterResolution{},
		}, nil
	}

	// Extract preset settings from workflow node
	presetSettings := make(map[string]interface{})
	for key, value := range toolDef.ToolExecutor.WorkflowNode.IntegrationSettings {
		presetSettings[key] = value
	}

	// Create resolution context
	ctx := ParameterResolutionContext{
		WorkflowNode:        toolDef.ToolExecutor.WorkflowNode,
		PresetSettings:      presetSettings,
		AIProvidedArguments: toolCall.Arguments,
		ToolName:            toolCall.Name,
	}

	// Resolve parameters
	result, err := m.parameterResolver.ResolveParameters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve parameters for tool %s: %w", toolCall.Name, err)
	}

	// Log resolution summary
	summary := m.parameterResolver.GetResolutionSummary(result)
	log.Debug().
		Str("tool_name", toolCall.Name).
		Str("resolution_summary", summary).
		Msg("Parameter resolution completed")

	return result, nil
}

// createIntegrationInputWithResolvedParams creates integration input with resolved parameters
func (m *DefaultToolCallManager) createIntegrationInputWithResolvedParams(
	toolCall domain.ToolCall,
	toolDef *ToolDefinition,
	resolvedParams *ParameterResolutionResult,
) (domain.IntegrationInput, error) {
	// Create a tool call item (similar to existing approach)
	toolCallItem := map[string]interface{}{
		"_tool_call":    true,
		"_tool_name":    toolCall.Name,
		"_tool_call_id": toolCall.ID,
	}

	// Add resolved parameters to the item (not just AI arguments)
	for key, value := range resolvedParams.ResolvedSettings {
		toolCallItem[key] = value
	}

	// Create payload with the tool call item
	toolCallItems := []interface{}{toolCallItem}
	payloadJSON, err := json.Marshal(toolCallItems)
	if err != nil {
		return domain.IntegrationInput{}, fmt.Errorf("failed to marshal tool call items: %w", err)
	}

	// Create integration input with resolved parameters
	integrationInput := domain.IntegrationInput{
		ActionType: toolDef.ActionType,
		InputJSON:  []byte("{}"), // Tool calls don't need input JSON
		PayloadByInputID: map[string]domain.Payload{
			"tool_call_input": domain.Payload(payloadJSON),
		},
		IntegrationParams: domain.IntegrationParams{
			Settings: resolvedParams.ResolvedSettings, // Use resolved settings instead of raw arguments
		},
		Workflow: &domain.Workflow{
			WorkspaceID: toolDef.ToolExecutor.WorkspaceID,
		},
	}

	log.Debug().
		Str("tool_name", toolCall.Name).
		Interface("resolved_item", toolCallItem).
		Int("resolved_params_count", len(resolvedParams.ResolvedSettings)).
		Msg("Created integration input with resolved parameters")

	return integrationInput, nil
}

// formatToolResult formats the integration output for tool result
func (m *DefaultToolCallManager) formatToolResult(output domain.IntegrationOutput) interface{} {
	// Try to parse ResultJSON if available
	if len(output.ResultJSONByOutputID) > 0 {
		resultJSON := output.ResultJSONByOutputID[0]

		if len(resultJSON) > 0 {
			var result interface{}
			if err := json.Unmarshal(resultJSON, &result); err == nil {
				return result
			}
			// If parsing fails, return as string
			return string(resultJSON)
		}
	}

	// Fallback to basic output structure
	return map[string]interface{}{
		"success": true,
		"message": "Tool executed successfully",
	}
}

// publishToolExecutionStartedEvent publishes a NodeExecutionStartedEvent for tool call
func (m *DefaultToolCallManager) publishToolExecutionStartedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	nodeID string,
) error {
	event := &domain.NodeExecutionStartedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              nodeID,
		Timestamp:           time.Now().UnixNano(),
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// publishToolExecutionCompletedEvent publishes a NodeExecutedEvent for successful tool call
func (m *DefaultToolCallManager) publishToolExecutionCompletedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	nodeID string,
	resolvedParams *ParameterResolutionResult,
	output domain.IntegrationOutput,
) error {
	// Build input items from resolved parameters
	inputItems, err := m.buildInputItemsFromParameters(resolvedParams)
	if err != nil {
		log.Error().Err(err).Msg("Failed to build input items for tool execution event")
		inputItems = make(map[string]domain.NodeItems)
	}

	// Build output items from integration output
	outputItems, err := m.buildOutputItemsFromIntegrationOutput(output, nodeID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to build output items for tool execution event")
		outputItems = make(map[string]domain.NodeItems)
	}

	event := &domain.NodeExecutedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              nodeID,
		ItemsByInputID:      inputItems,
		ItemsByOutputID:     outputItems,
		Timestamp:           time.Now().UnixNano(),
		ExecutionOrder:      0, // OrderedEventPublisher will handle ordering via EventOrder field
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// publishToolExecutionFailedEvent publishes a NodeFailedEvent for failed tool call
func (m *DefaultToolCallManager) publishToolExecutionFailedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	nodeID string,
	toolCall domain.ToolCall,
	resolvedParams *ParameterResolutionResult,
	err error,
) error {
	// Build input items from resolved parameters
	inputItems, itemsErr := m.buildInputItemsFromParameters(resolvedParams)
	if itemsErr != nil {
		log.Error().Err(itemsErr).Msg("Failed to build input items for tool execution failed event")
		inputItems = make(map[string]domain.NodeItems)
	}

	event := &domain.NodeFailedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              nodeID,
		Error:               err.Error(),
		ExecutionOrder:      0, // OrderedEventPublisher will handle ordering via EventOrder field
		Timestamp:           time.Now().UnixNano(),
		ItemsByInputID:      inputItems,
		ItemsByOutputID:     make(map[string]domain.NodeItems), // No output on failure
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// buildInputItemsFromParameters converts resolved parameters to NodeItems format
func (m *DefaultToolCallManager) buildInputItemsFromParameters(resolvedParams *ParameterResolutionResult) (map[string]domain.NodeItems, error) {
	if resolvedParams == nil {
		return make(map[string]domain.NodeItems), nil
	}

	// Convert resolved settings to items format
	items := []domain.Item{}
	if len(resolvedParams.ResolvedSettings) > 0 {
		items = append(items, domain.Item(resolvedParams.ResolvedSettings))
	}

	inputID := "tool_call_input"
	return map[string]domain.NodeItems{
		inputID: {
			FromNodeID: m.AgentNodeID, // Tool calls originate from AI agent
			Items:      items,
		},
	}, nil
}

// buildOutputItemsFromIntegrationOutput converts integration output to NodeItems format
func (m *DefaultToolCallManager) buildOutputItemsFromIntegrationOutput(output domain.IntegrationOutput, fromNodeID string) (map[string]domain.NodeItems, error) {
	outputItems := make(map[string]domain.NodeItems)

	// Convert each output payload to items
	for outputIndex, payload := range output.ResultJSONByOutputID {
		items, err := payload.ToItems()
		if err != nil {
			log.Error().Err(err).Int("output_index", outputIndex).Msg("Failed to convert payload to items")
			continue
		}

		outputID := fmt.Sprintf("output-%s-%d", fromNodeID, outputIndex)
		outputItems[outputID] = domain.NodeItems{
			FromNodeID: fromNodeID,
			Items:      items,
		}
	}

	return outputItems, nil
}

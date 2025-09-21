package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"

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
	Name                string                       `json:"name"`
	Description         string                       `json:"description"`
	Parameters          map[string]interface{}       `json:"parameters"`
	ActionType          domain.IntegrationActionType `json:"action_type"`
	ToolExecutor        *ToolExecutor                `json:"tool_executor"`
	IntegrationSettings map[string]interface{}       `json:"integration_settings"`
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

	// GetPeekableData retrieves peekable data for all tools that have agent-provided peekable fields
	GetPeekableData(ctx context.Context, toolDefinitions []ToolDefinition) map[string]map[string][]domain.PeekResultItem
}

// DefaultToolCallManager implements ToolCallManager
type DefaultToolCallManager struct {
	AgentNodeID                string
	executorIntegrationManager domain.ExecutorIntegrationManager
	parameterBinder            domain.IntegrationParameterBinder
	parameterResolver          *AIParameterResolver
	peekableResolver           *PeekableValueResolver
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
		peekableResolver:           NewPeekableValueResolver(deps.ExecutorIntegrationManager),
		eventPublisher:             deps.EventPublisher,
	}
}

// DiscoverTools extracts all available tools from the provided executors
func (m *DefaultToolCallManager) DiscoverTools(ctx context.Context, toolExecutors []ToolExecutor) ([]ToolDefinition, error) {
	var allTools []ToolDefinition

	for _, toolExec := range toolExecutors {

		integration, err := m.executorIntegrationManager.GetIntegration(ctx, toolExec.IntegrationType)
		if err != nil {
			log.Error().
				Err(err).
				Str("integration_type", string(toolExec.IntegrationType)).
				Msg("Failed to get integration schema")
			continue
		}

		for _, action := range integration.Actions {
			if len(toolExec.AllowedActions) > 0 {
				allowed := false
				for _, allowedAction := range toolExec.AllowedActions {
					if action.ActionType == allowedAction {
						allowed = true
						break
					}
				}
				if !allowed {
					continue
				}
			}

			toolDef := m.convertActionToTool(action, &toolExec)
			allTools = append(allTools, toolDef)

		}
	}

	return allTools, nil
}

// convertActionToTool converts an IntegrationAction to a ToolDefinition
func (m *DefaultToolCallManager) convertActionToTool(action domain.IntegrationAction, toolExec *ToolExecutor) ToolDefinition {
	toolName := fmt.Sprintf("%s_%s", strings.ToLower(string(toolExec.IntegrationType)), strings.ToLower(string(action.ActionType)))

	properties := make(map[string]interface{})
	required := []string{}

	presetSettings := make(map[string]interface{})
	if toolExec.WorkflowNode != nil {
		for key, value := range toolExec.WorkflowNode.IntegrationSettings {
			presetSettings[key] = value
		}
	}

	for _, prop := range action.Properties {
		propSchema := m.convertNodePropertyToSchema(prop)
		properties[prop.Key] = propSchema

		if prop.Required {
			required = append(required, prop.Key)
		}
	}

	parameters := map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}

	return ToolDefinition{
		Name:                toolName,
		Description:         fmt.Sprintf("%s: %s", action.Name, action.Description),
		Parameters:          parameters,
		ActionType:          action.ActionType,
		ToolExecutor:        toolExec,
		IntegrationSettings: presetSettings,
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

	if prop.Type == domain.NodePropertyType_Array {
		if prop.ArrayOpts != nil {
			itemsSchema := map[string]interface{}{
				"type": m.mapNodePropertyType(prop.ArrayOpts.ItemType),
			}

			if len(prop.ArrayOpts.ItemProperties) > 0 {
				itemsProperties := make(map[string]interface{})
				itemsRequired := []string{}

				for _, itemProp := range prop.ArrayOpts.ItemProperties {
					itemsProperties[itemProp.Key] = m.convertNodePropertyToSchema(itemProp)
					if itemProp.Required {
						itemsRequired = append(itemsRequired, itemProp.Key)
					}
				}

				itemsSchema["properties"] = itemsProperties
				if len(itemsRequired) > 0 {
					itemsSchema["required"] = itemsRequired
				}
			}

			schema["items"] = itemsSchema

			if prop.ArrayOpts.MinItems > 0 {
				schema["minItems"] = prop.ArrayOpts.MinItems
			}
			if prop.ArrayOpts.MaxItems > 0 {
				schema["maxItems"] = prop.ArrayOpts.MaxItems
			}
		} else {
			schema["items"] = map[string]interface{}{
				"type": "string",
			}
		}
	}

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
	case domain.NodePropertyType_String, domain.NodePropertyType_Text, domain.NodePropertyType_CodeEditor:
		return "string"
	case domain.NodePropertyType_Integer:
		return "integer"
	case domain.NodePropertyType_Number:
		return "number"
	case domain.NodePropertyType_Float:
		return "number"
	case domain.NodePropertyType_Boolean:
		return "boolean"
	case domain.NodePropertyType_Array, domain.NodePropertyType_TagInput:
		return "array"
	case domain.NodePropertyType_Map:
		return "object"
	case domain.NodePropertyType_Date:
		return "string"
	default:
		return "string"
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

func (m *DefaultToolCallManager) GetPeekableData(ctx context.Context, toolDefinitions []ToolDefinition) map[string]map[string][]domain.PeekResultItem {
	result := make(map[string]map[string][]domain.PeekResultItem)

	for _, toolDef := range toolDefinitions {
		if toolDef.ToolExecutor.WorkflowNode == nil {
			continue
		}

		// Identify peekable fields for this tool
		peekableFields, err := m.peekableResolver.IdentifyPeekableFields(
			toolDef.ToolExecutor.WorkflowNode,
			toolDef.ToolExecutor.IntegrationType,
			toolDef.ActionType,
		)
		if err != nil || len(peekableFields) == 0 {
			continue
		}

		toolPeekables := make(map[string][]domain.PeekResultItem)

		// Get peekable data for each field
		for fieldKey, peekableType := range peekableFields {
			// Check if the executor implements IntegrationPeeker
			peeker, ok := toolDef.ToolExecutor.Executor.(domain.IntegrationPeeker)
			if !ok {
				continue
			}

			// Prepare peek parameters
			peekParams := domain.PeekParams{
				PeekableType: peekableType,
				WorkspaceID:  toolDef.ToolExecutor.WorkspaceID,
				UserID:       "",           // Will be set by the integration if needed
				PayloadJSON:  []byte("{}"), // Default empty payload
			}

			// Perform the peek operation
			peekResult, err := peeker.Peek(ctx, peekParams)
			if err != nil {
				continue
			}

			toolPeekables[fieldKey] = peekResult.Result
		}

		if len(toolPeekables) > 0 {
			result[toolDef.Name] = toolPeekables
		}
	}

	return result
}

// ExecuteToolCall executes a tool call from LLM and returns the result
func (m *DefaultToolCallManager) ExecuteToolCall(ctx context.Context, toolCall domain.ToolCall, toolDefinitions []ToolDefinition) (*ToolCallResult, error) {
	startTime := time.Now()

	result := &ToolCallResult{
		ToolName:  toolCall.Name,
		StartTime: startTime,
	}

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

	workflowCtx, hasWorkflowCtx := domain.GetWorkflowExecutionContext(ctx)

	if hasWorkflowCtx && workflowCtx.EnableEvents {
		err = m.publishToolExecutionStartedEvent(ctx, workflowCtx, toolNodeID)
		if err != nil {
			log.Error().Err(err).Str("tool_name", toolCall.Name).Msg("Failed to publish tool execution started event")
		}
	}

	resolvedParams, err := m.resolveToolParameters(ctx, toolCall, toolDef)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Parameter resolution failed: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	integrationInput, err := m.createIntegrationInputWithResolvedParams(toolCall, toolDef, resolvedParams)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	output, err := toolDef.ToolExecutor.Executor.Execute(ctx, integrationInput)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Success = false
		result.Error = err.Error()

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

	if hasWorkflowCtx && workflowCtx.EnableEvents {
		err = m.publishToolExecutionCompletedEvent(ctx, workflowCtx, toolNodeID, resolvedParams, output)
		if err != nil {
			log.Error().Err(err).Str("tool_name", toolCall.Name).Msg("Failed to publish tool execution completed event")
		}
	}

	return result, nil
}

// resolveToolParameters resolves tool parameters using AI vs preset values
func (m *DefaultToolCallManager) resolveToolParameters(ctx context.Context, toolCall domain.ToolCall, toolDef *ToolDefinition) (*ParameterResolutionResult, error) {
	if toolDef.ToolExecutor.WorkflowNode == nil {

		return &ParameterResolutionResult{
			ResolvedSettings: toolCall.Arguments,
			ResolutionLog:    []ParameterResolution{},
		}, nil
	}

	presetSettings := make(map[string]interface{})
	for key, value := range toolDef.ToolExecutor.WorkflowNode.IntegrationSettings {
		presetSettings[key] = value
	}

	resolutionCtx := ParameterResolutionContext{
		WorkflowNode:        toolDef.ToolExecutor.WorkflowNode,
		PresetSettings:      presetSettings,
		AIProvidedArguments: toolCall.Arguments,
		ToolName:            toolCall.Name,
	}

	result, err := m.parameterResolver.ResolveParameters(resolutionCtx)

	if err != nil {
		return nil, fmt.Errorf("failed to resolve parameters for tool %s: %w", toolCall.Name, err)
	}

	err = m.resolvePeekableValues(ctx, result, toolDef)
	if err != nil {
		log.Error().
			Err(err).
			Str("tool_name", toolCall.Name).
			Msg("Failed to resolve peekable values, continuing with unresolved values")
	}

	return result, nil
}

// createIntegrationInputWithResolvedParams creates integration input with resolved parameters
func (m *DefaultToolCallManager) createIntegrationInputWithResolvedParams(
	toolCall domain.ToolCall,
	toolDef *ToolDefinition,
	resolvedParams *ParameterResolutionResult,
) (domain.IntegrationInput, error) {
	toolCallItem := map[string]interface{}{
		"_tool_call":    true,
		"_tool_name":    toolCall.Name,
		"_tool_call_id": toolCall.ID,
	}

	for key, value := range resolvedParams.ResolvedSettings {
		toolCallItem[key] = value
	}

	toolCallItems := []interface{}{toolCallItem}
	payloadJSON, err := json.Marshal(toolCallItems)
	if err != nil {
		return domain.IntegrationInput{}, fmt.Errorf("failed to marshal tool call items: %w", err)
	}

	integrationInput := domain.IntegrationInput{
		ActionType: toolDef.ActionType,
		InputJSON:  []byte("{}"),
		PayloadByInputID: map[string]domain.Payload{
			"tool_call_input": domain.Payload(payloadJSON),
		},
		IntegrationParams: domain.IntegrationParams{
			Settings: resolvedParams.ResolvedSettings,
		},
		Workflow: &domain.Workflow{
			WorkspaceID: toolDef.ToolExecutor.WorkspaceID,
		},
	}

	return integrationInput, nil
}

// formatToolResult formats the integration output for tool result
func (m *DefaultToolCallManager) formatToolResult(output domain.IntegrationOutput) interface{} {
	if len(output.ResultJSONByOutputID) > 0 {
		resultJSON := output.ResultJSONByOutputID[0]

		if len(resultJSON) > 0 {
			var result interface{}
			if err := json.Unmarshal(resultJSON, &result); err == nil {
				return result
			}
			return string(resultJSON)
		}
	}

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
	inputItems, err := m.buildInputItemsFromParameters(resolvedParams)
	if err != nil {
		log.Error().Err(err).Msg("Failed to build input items for tool execution event")
		inputItems = make(map[string]domain.NodeItems)
	}

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

// resolvePeekableValues resolves peekable field values from display text to actual IDs
func (m *DefaultToolCallManager) resolvePeekableValues(ctx context.Context, resolvedParams *ParameterResolutionResult, toolDef *ToolDefinition) error {
	if toolDef.ToolExecutor.WorkflowNode == nil {
		return nil // No workflow node, skip peekable resolution
	}

	peekableFields, err := m.peekableResolver.IdentifyPeekableFields(
		toolDef.ToolExecutor.WorkflowNode,
		toolDef.ToolExecutor.IntegrationType,
		toolDef.ActionType,
	)
	if err != nil {
		return fmt.Errorf("failed to identify peekable fields: %w", err)
	}

	if len(peekableFields) == 0 {
		return nil // No peekable fields to resolve
	}

	for fieldKey, peekableType := range peekableFields {
		fieldValue, exists := resolvedParams.ResolvedSettings[fieldKey]
		if !exists || fieldValue == nil {
			continue
		}

		if fieldValueStr := fmt.Sprintf("%v", fieldValue); fieldValueStr == "" || fieldValueStr == "<nil>" {
			continue
		}

		wasProvidedByAgent := false
		for _, resolution := range resolvedParams.ResolutionLog {
			if resolution.ProvidedByAgent && (resolution.Path == fieldKey || strings.HasSuffix(resolution.Path, "."+fieldKey)) {
				wasProvidedByAgent = true
				break
			}
		}

		if !wasProvidedByAgent {
			continue
		}

		resolvedValue, err := m.performPeekableResolution(ctx, fieldValue, peekableType, toolDef)

		if err != nil {
			log.Error().
				Err(err).
				Str("field_key", fieldKey).
				Interface("display_value", fieldValue).
				Msg("Failed to resolve peekable value")
			continue
		}

		// Update the resolved value if it changed
		if resolvedValue != fieldValue {
			resolvedParams.ResolvedSettings[fieldKey] = resolvedValue

			// Update resolution log
			for i, res := range resolvedParams.ResolutionLog {
				if res.Path == fieldKey || strings.HasSuffix(res.Path, "."+fieldKey) {
					resolvedParams.ResolutionLog[i].Value = resolvedValue
					break
				}
			}
		}
	}

	return nil
}

// performPeekableResolution performs the actual peekable resolution using the tool executor
func (m *DefaultToolCallManager) performPeekableResolution(ctx context.Context, displayValue interface{}, peekableType domain.IntegrationPeekableType, toolDef *ToolDefinition) (interface{}, error) {
	// Convert display value to string for processing
	displayStr := fmt.Sprintf("%v", displayValue)
	if displayStr == "" || displayStr == "<nil>" {
		return displayValue, nil // Return original if empty
	}

	// Check if the tool executor supports peeking
	peeker, ok := toolDef.ToolExecutor.Executor.(domain.IntegrationPeeker)
	if !ok {
		return displayValue, nil
	}

	// Prepare peek parameters
	peekParams := domain.PeekParams{
		PeekableType: peekableType,
		WorkspaceID:  toolDef.ToolExecutor.WorkspaceID,
		UserID:       "",           // Will be set by the integration if needed
		PayloadJSON:  []byte("{}"), // Default empty payload
	}

	// Perform the peek operation
	peekResult, err := peeker.Peek(ctx, peekParams)
	if err != nil {
		log.Error().
			Err(err).
			Str("peekable_type", string(peekableType)).
			Msg("Peekable API call failed")
		return displayValue, fmt.Errorf("peekable lookup failed: %w", err)
	}

	// Debug: Log what peekable API returned
	// Process peekable results

	// Search for matching value in peek results
	resolvedValue := m.findMatchingPeekValue(displayStr, peekResult.Result)
	if resolvedValue != "" {
		return resolvedValue, nil
	}

	// If still no match, return the display value as-is (it might be a valid ID already)
	return displayValue, nil
}

// findMatchingPeekValue searches for an exact match in peek results
func (m *DefaultToolCallManager) findMatchingPeekValue(displayValue string, peekResults []domain.PeekResultItem) string {
	displayLower := strings.ToLower(displayValue)

	for _, item := range peekResults {
		// Check if display value matches the content (display text)
		if strings.ToLower(item.Content) == displayLower {
			return item.Value // Return the actual ID, not the display name
		}

		// Check if display value matches the value (already an ID)
		if strings.ToLower(item.Value) == displayLower {
			return item.Value
		}

		// Check if display value IS the key (filename match)
		if strings.ToLower(item.Key) == displayLower {
			return item.Value // Return the actual ID, not the display name
		}

		// For file names, check if it's part of the content (handle extensions)
		if strings.Contains(strings.ToLower(item.Content), displayLower) {
			return item.Value // Return the actual ID, not the display name
		}
	}

	return ""
}

func (m *DefaultToolCallManager) findActionByType(actions []domain.IntegrationAction, actionType domain.IntegrationActionType) *domain.IntegrationAction {
	for _, action := range actions {
		if action.ActionType == actionType {
			return &action
		}
	}
	return nil
}

func (m *DefaultToolCallManager) findPropertyByKey(properties []domain.NodeProperty, key string) *domain.NodeProperty {
	for _, prop := range properties {
		if prop.Key == key {
			return &prop
		}
	}
	return nil
}

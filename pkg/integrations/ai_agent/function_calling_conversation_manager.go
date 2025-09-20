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

const (
	maxFunctionCallingRounds = 10
	functionCallTimeout      = 60 * time.Second
)

// FunctionCallingStep represents a single step in the function calling pattern
type FunctionCallingStep string

const (
	StepLLMCall       FunctionCallingStep = "llm_call"
	StepToolExecution FunctionCallingStep = "tool_execution"
	StepCompleted     FunctionCallingStep = "completed"
)

// FunctionCallingExecutionStatus represents the current status of the conversation
type FunctionCallingExecutionStatus string

const (
	FCStatusRunning   FunctionCallingExecutionStatus = "running"
	FCStatusCompleted FunctionCallingExecutionStatus = "completed"
	FCStatusFailed    FunctionCallingExecutionStatus = "failed"
	FCStatusPaused    FunctionCallingExecutionStatus = "paused"
)

// FunctionCallingExecutionError represents an error during execution
type FunctionCallingExecutionError struct {
	Type        string              `json:"type"`
	Message     string              `json:"message"`
	Round       int                 `json:"round"`
	Step        FunctionCallingStep `json:"step"`
	Recoverable bool                `json:"recoverable"`
	Timestamp   time.Time           `json:"timestamp"`
}

// FunctionCallingState represents the current state of a function calling conversation
type FunctionCallingState struct {
	ConversationID      string                         `json:"conversation_id"`
	WorkspaceID         string                         `json:"workspace_id"`
	CurrentStep         FunctionCallingStep            `json:"current_step"`
	Round               int                            `json:"round"`
	ConversationHistory []domain.ConversationMessage   `json:"conversation_history"`
	ToolExecutions      []FunctionCallExecution        `json:"tool_executions"`
	Context             map[string]interface{}         `json:"context"`
	Status              FunctionCallingExecutionStatus `json:"status"`
	LastError           *FunctionCallingExecutionError `json:"last_error,omitempty"`
	ToolFailures        int                            `json:"tool_failures"`
	CreatedAt           time.Time                      `json:"created_at"`
	UpdatedAt           time.Time                      `json:"updated_at"`
}

// FunctionCallExecution represents an executed function call
type FunctionCallExecution struct {
	Round      int                    `json:"round"`
	ToolCallID string                 `json:"tool_call_id"`
	ToolName   string                 `json:"tool_name"`
	Parameters map[string]interface{} `json:"parameters"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	Success    bool                   `json:"success"`
	Result     interface{}            `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Duration   time.Duration          `json:"duration"`
	Metadata   *ToolExecutionMetadata `json:"metadata,omitempty"`
}

// ToolExecutionMetadata contains additional metadata about tool execution
type ToolExecutionMetadata struct {
	NodeID          string                        `json:"node_id"`
	IntegrationType domain.IntegrationType        `json:"integration_type"`
	ActionType      domain.IntegrationActionType `json:"action_type"`
}

// FunctionCallingConversationManager manages function calling pattern conversations
type FunctionCallingConversationManager struct {
	agentNodeID     string
	llm             domain.IntegrationLLM
	memory          domain.IntegrationMemory
	toolExecutors   []ToolExecutor
	toolCallManager ToolCallManager
	toolDefinitions []ToolDefinition
	stateManager    FunctionCallingStateManager
	errorHandler    *FunctionCallingErrorHandler
	eventPublisher  domain.EventPublisher
	executeParams   ExecuteParams
	llmNodeParams   LLMNodeParams
	memoryManager   ConversationMemoryManager
}

// FunctionCallingStateManager handles state persistence and recovery for function calling
type FunctionCallingStateManager interface {
	SaveState(ctx context.Context, state *FunctionCallingState) error
	LoadState(ctx context.Context, conversationID string) (*FunctionCallingState, error)
	DeleteState(ctx context.Context, conversationID string) error
}

// FunctionCallingErrorHandler handles errors and recovery strategies
type FunctionCallingErrorHandler struct {
	maxRetries int
	retryDelay time.Duration
}

type FunctionCallingConversationManagerDeps struct {
	AgentNodeID     string
	LLM             domain.IntegrationLLM
	Memory          domain.IntegrationMemory
	ToolExecutors   []ToolExecutor
	ToolCallManager ToolCallManager
	StateManager    FunctionCallingStateManager
	EventPublisher  domain.EventPublisher
	ExecuteParams   ExecuteParams
	MemoryManager   ConversationMemoryManager
	LLMNodeParams   LLMNodeParams
}

// NewFunctionCallingConversationManager creates a new function calling conversation manager
func NewFunctionCallingConversationManager(deps FunctionCallingConversationManagerDeps) *FunctionCallingConversationManager {
	return &FunctionCallingConversationManager{
		agentNodeID:     deps.AgentNodeID,
		llm:             deps.LLM,
		memory:          deps.Memory,
		toolExecutors:   deps.ToolExecutors,
		toolCallManager: deps.ToolCallManager,
		stateManager:    deps.StateManager,
		errorHandler:    NewFunctionCallingErrorHandler(),
		eventPublisher:  deps.EventPublisher,
		executeParams:   deps.ExecuteParams,
		memoryManager:   deps.MemoryManager,
		llmNodeParams:   deps.LLMNodeParams,
	}
}

// NewFunctionCallingErrorHandler creates a new error handler
func NewFunctionCallingErrorHandler() *FunctionCallingErrorHandler {
	return &FunctionCallingErrorHandler{
		maxRetries: 3,
		retryDelay: time.Second * 2,
	}
}

// ExecuteFunctionCallingConversation executes a full function calling conversation
func (f *FunctionCallingConversationManager) ExecuteFunctionCallingConversation(
	ctx context.Context,
	conversationID, workspaceID, initialPrompt string,
) (*ConversationResult, error) {
	// Discover tools first
	toolDefinitions, err := f.toolCallManager.DiscoverTools(ctx, f.toolExecutors)
	if err != nil {
		return nil, fmt.Errorf("failed to discover tools: %w", err)
	}
	f.toolDefinitions = toolDefinitions

	// Initialize or load state
	state, err := f.initializeState(ctx, conversationID, workspaceID, initialPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state: %w", err)
	}

	defer f.saveStateWithRetry(ctx, state)

	log.Info().
		Str("conversation_id", conversationID).
		Str("workspace_id", workspaceID).
		Msg("Starting function calling conversation")

	// Store conversation start in memory
	err = f.memoryManager.StoreConversationStart(ctx, conversationID, workspaceID, initialPrompt)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store conversation start in memory")
		// Don't fail the conversation due to memory issues
	}

	// Execute function calling loop
	result, err := f.executeFunctionCallingLoop(ctx, state)
	if err != nil {
		state.Status = FCStatusFailed
		state.LastError = &FunctionCallingExecutionError{
			Type:        "execution_error",
			Message:     err.Error(),
			Round:       state.Round,
			Step:        state.CurrentStep,
			Recoverable: false,
			Timestamp:   time.Now(),
		}

		// Store failed conversation in memory
		storeErr := f.memoryManager.StoreConversationComplete(ctx, state, nil, workspaceID, initialPrompt)
		if storeErr != nil {
			log.Error().Err(storeErr).Msg("Failed to store failed conversation in memory")
		}

		return nil, err
	}

	state.Status = FCStatusCompleted

	// Store completed conversation in memory
	err = f.memoryManager.StoreConversationComplete(ctx, state, result, workspaceID, initialPrompt)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store completed conversation in memory")
		// Don't fail the conversation due to memory issues
	}

	return result, nil
}

// initializeState initializes or loads conversation state
func (f *FunctionCallingConversationManager) initializeState(
	ctx context.Context,
	conversationID, workspaceID, initialPrompt string,
) (*FunctionCallingState, error) {
	// Try to load existing state
	if existingState, err := f.stateManager.LoadState(ctx, conversationID); err == nil {
		log.Info().
			Str("conversation_id", conversationID).
			Msg("Resuming existing function calling conversation")
		return existingState, nil
	}

	// Create new state
	state := &FunctionCallingState{
		ConversationID:      conversationID,
		WorkspaceID:         workspaceID,
		CurrentStep:         StepLLMCall,
		Round:               0,
		ConversationHistory: make([]domain.ConversationMessage, 0),
		ToolExecutions:      make([]FunctionCallExecution, 0),
		Context:             make(map[string]interface{}),
		Status:              FCStatusRunning,
		ToolFailures:        0,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	// Store initial prompt and available tools in context
	state.Context["initial_prompt"] = initialPrompt
	state.Context["available_tools"] = f.getToolNames()

	// Add initial user message to conversation history
	state.ConversationHistory = append(state.ConversationHistory, domain.ConversationMessage{
		Role:    "user",
		Content: initialPrompt,
	})

	return state, nil
}

// executeFunctionCallingLoop executes the main function calling loop
func (f *FunctionCallingConversationManager) executeFunctionCallingLoop(
	ctx context.Context,
	state *FunctionCallingState,
) (*ConversationResult, error) {
	for state.Round < maxFunctionCallingRounds && state.Status == FCStatusRunning {
		log.Debug().
			Str("conversation_id", state.ConversationID).
			Int("round", state.Round).
			Str("step", string(state.CurrentStep)).
			Msg("Executing function calling step")

		switch state.CurrentStep {
		case StepLLMCall:
			if err := f.executeLLMCall(ctx, state); err != nil {
				if f.shouldContinueAfterError(state, err) {
					state.CurrentStep = StepCompleted
					continue
				}
				return nil, fmt.Errorf("LLM call failed: %w", err)
			}

		case StepToolExecution:
			if err := f.executeToolCalls(ctx, state); err != nil {
				if f.shouldContinueAfterError(state, err) {
					state.CurrentStep = StepCompleted
					continue
				}
				return nil, fmt.Errorf("tool execution failed: %w", err)
			}

		case StepCompleted:
			// Conversation is complete
			return f.buildConversationResult(state), nil
		}

		state.UpdatedAt = time.Now()

		// Save state periodically
		if err := f.stateManager.SaveState(ctx, state); err != nil {
			log.Error().Err(err).Msg("Failed to save state")
		}
	}

	// Handle max rounds exceeded
	if state.Round >= maxFunctionCallingRounds {
		log.Warn().
			Str("conversation_id", state.ConversationID).
			Int("rounds", state.Round).
			Msg("Function calling conversation exceeded max rounds")

		return f.buildConversationResult(state), nil
	}

	return f.buildConversationResult(state), nil
}

// executeLLMCall executes an LLM call with tools
func (f *FunctionCallingConversationManager) executeLLMCall(ctx context.Context, state *FunctionCallingState) error {
	callCtx, cancel := context.WithTimeout(ctx, functionCallTimeout)
	defer cancel()

	// Get LLM-compatible tools
	llmTools := f.toolCallManager.GetLLMTools(f.toolDefinitions)

	// Build system prompt with memory context
	systemPrompt := f.buildSystemPrompt(ctx, state.WorkspaceID)

	// Create GenerateRequest for the LLM
	req := domain.GenerateRequest{
		Messages:     state.ConversationHistory,
		Tools:        llmTools,
		SystemPrompt: systemPrompt,
		Temperature:  f.llmNodeParams.Temperature,
		MaxTokens:    f.llmNodeParams.MaxTokens,
		Model:        f.llmNodeParams.Model,
	}

	// Log the request for debugging
	log.Debug().
		Str("conversation_id", state.ConversationID).
		Int("message_count", len(state.ConversationHistory)).
		Int("tool_count", len(llmTools)).
		Int("system_prompt_length", len(systemPrompt)).
		Msg("Making LLM request")

	// Call the LLM with event publishing
	response, err := f.generateWithConversationAndEvents(callCtx, req)
	if err != nil {
		return fmt.Errorf("LLM call failed: %w", err)
	}

	// Add assistant response to conversation history
	assistantMessage := domain.ConversationMessage{
		Role:      "assistant",
		Content:   response.Content,
		ToolCalls: response.ToolCalls,
	}
	state.ConversationHistory = append(state.ConversationHistory, assistantMessage)

	// Determine next step
	if len(response.ToolCalls) > 0 {
		state.CurrentStep = StepToolExecution
		log.Debug().
			Str("conversation_id", state.ConversationID).
			Int("tool_calls", len(response.ToolCalls)).
			Msg("LLM requested tool calls")

		// Log tool calls for debugging
		for i, toolCall := range response.ToolCalls {
			log.Debug().
				Str("conversation_id", state.ConversationID).
				Int("tool_index", i).
				Str("tool_name", toolCall.Name).
				Interface("arguments", toolCall.Arguments).
				Msg("LLM tool call request")
		}
	} else {
		state.CurrentStep = StepCompleted
		log.Debug().
			Str("conversation_id", state.ConversationID).
			Str("response_content", response.Content).
			Msg("LLM provided final response")
	}

	return nil
}

// executeToolCalls executes all tool calls from the LLM response
func (f *FunctionCallingConversationManager) executeToolCalls(ctx context.Context, state *FunctionCallingState) error {
	// Get the last assistant message with tool calls
	if len(state.ConversationHistory) == 0 {
		return fmt.Errorf("no conversation history available")
	}

	lastMessage := state.ConversationHistory[len(state.ConversationHistory)-1]
	if len(lastMessage.ToolCalls) == 0 {
		return fmt.Errorf("no tool calls found in last message")
	}

	toolResults := make([]domain.ToolResult, 0, len(lastMessage.ToolCalls))

	// Execute each tool call
	for _, toolCall := range lastMessage.ToolCalls {
		execution := &FunctionCallExecution{
			Round:      state.Round,
			ToolCallID: toolCall.ID,
			ToolName:   toolCall.Name,
			Parameters: toolCall.Arguments,
			StartTime:  time.Now(),
		}

		// Find tool metadata from tool executors
		for _, toolExecutor := range f.toolExecutors {
			expectedToolName := fmt.Sprintf("%s_%s", toolExecutor.IntegrationType, toolExecutor.WorkflowNode.ActionType)
			if toolCall.Name == expectedToolName {
				execution.Metadata = &ToolExecutionMetadata{
					NodeID:          toolExecutor.NodeID,
					IntegrationType: toolExecutor.IntegrationType,
					ActionType:      domain.IntegrationActionType(toolExecutor.WorkflowNode.ActionType),
				}
				break
			}
		}

		result, err := f.toolCallManager.ExecuteToolCall(ctx, toolCall, f.toolDefinitions)
		execution.EndTime = time.Now()
		execution.Duration = execution.EndTime.Sub(execution.StartTime)

		if err != nil {
			execution.Success = false
			execution.Error = err.Error()
			state.ToolFailures++

			// Add error result
			toolResults = append(toolResults, domain.ToolResult{
				ToolCallID: toolCall.ID,
				Content:    fmt.Sprintf("Error executing tool: %s", err.Error()),
			})

			log.Error().
				Err(err).
				Str("tool_name", toolCall.Name).
				Msg("Tool execution failed")
		} else {
			execution.Success = true
			execution.Result = result.Result

			// Format tool result
			resultContent := f.formatToolResult(result)
			toolResults = append(toolResults, domain.ToolResult{
				ToolCallID: toolCall.ID,
				Content:    resultContent,
			})

			log.Debug().
				Str("tool_name", toolCall.Name).
				Dur("duration", execution.Duration).
				Msg("Tool execution completed successfully")
		}

		state.ToolExecutions = append(state.ToolExecutions, *execution)

		// Record in tool tracker if available
		if execution.Metadata != nil {
			if execContext, ok := domain.GetWorkflowExecutionContext(ctx); ok && execContext.ToolTracker != nil {
				toolExec := &domain.ToolExecution{
					Identifier: domain.ToolIdentifier{
						NodeID:          execution.Metadata.NodeID,
						IntegrationType: execution.Metadata.IntegrationType,
						ActionType:      execution.Metadata.ActionType,
					},
					ExecutedAt:  execution.StartTime,
					CompletedAt: execution.EndTime,
					Result:      execution.Result,
				}
				if !execution.Success && execution.Error != "" {
					toolExec.Error = fmt.Errorf(execution.Error)
				}
				execContext.ToolTracker.RecordExecution(toolExec)
			}
		}
	}

	// Add tool results to conversation history
	toolMessage := domain.ConversationMessage{
		Role:        "tool",
		ToolResults: toolResults,
	}
	state.ConversationHistory = append(state.ConversationHistory, toolMessage)

	// Continue to next LLM call or complete
	state.Round++
	state.CurrentStep = StepLLMCall

	return nil
}

// Helper methods

func (f *FunctionCallingConversationManager) buildSystemPrompt(ctx context.Context, workspaceID string) string {
	toolDescriptions := f.formatToolDescriptions()
	peekableContext := f.buildPeekableContext(ctx)

	// Use system prompt from LLM node parameters if available, otherwise use default
	var basePrompt string
	if f.llmNodeParams.SystemPrompt != "" {
		basePrompt = f.llmNodeParams.SystemPrompt
	} else {
		basePrompt = fmt.Sprintf(`You are an AI assistant that can use tools to help users complete tasks.

Available tools:
%s

%sInstructions:
1. Always use tools to complete user requests - don't just provide explanations
2. For missing required parameters, extract values from the user's message (e.g., file names, descriptions)
3. Call the tool even if you think information is missing - the system will handle parameter resolution
4. Pre-configured parameters are automatically filled
5. Provide a clear response about what you accomplished

IMPORTANT: Always call tools to perform actions. Extract any identifiers from the user's message as parameter values.`, toolDescriptions, peekableContext)
	}

	// If using custom system prompt, append tool descriptions
	if f.llmNodeParams.SystemPrompt != "" {
		basePrompt = fmt.Sprintf(`%s

Available tools:
%s

%s`, basePrompt, toolDescriptions, peekableContext)
	}

	// Enhance with memory context if available
	enhancedPrompt, err := f.memoryManager.EnhanceSystemPrompt(ctx, basePrompt, workspaceID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to enhance system prompt with memory")
		return basePrompt
	}

	return enhancedPrompt
}

func (f *FunctionCallingConversationManager) buildPeekableContext(ctx context.Context) string {
	if len(f.toolDefinitions) == 0 {
		return ""
	}

	peekableData := f.toolCallManager.GetPeekableData(ctx, f.toolDefinitions)
	if len(peekableData) == 0 {
		return ""
	}

	peekableInfo := f.formatPeekableInfo(peekableData)
	if len(peekableInfo) == 0 {
		return ""
	}

	return fmt.Sprintf(`Available data for selection:
%s

`, strings.Join(peekableInfo, "\n"))
}

func (f *FunctionCallingConversationManager) formatPeekableInfo(peekableData map[string]map[string][]domain.PeekResultItem) []string {
	var peekableInfo []string

	for toolName, toolPeekables := range peekableData {
		toolInfo := f.formatToolPeekables(toolName, toolPeekables)
		peekableInfo = append(peekableInfo, toolInfo...)
	}

	return peekableInfo
}

func (f *FunctionCallingConversationManager) formatToolPeekables(toolName string, toolPeekables map[string][]domain.PeekResultItem) []string {
	var toolInfo []string

	for fieldKey, peekResults := range toolPeekables {
		fieldInfo := f.formatFieldOptions(toolName, fieldKey, peekResults)
		if fieldInfo != "" {
			toolInfo = append(toolInfo, fieldInfo)
		}
	}

	return toolInfo
}

func (f *FunctionCallingConversationManager) formatFieldOptions(toolName, fieldKey string, peekResults []domain.PeekResultItem) string {
	options := f.extractOptions(peekResults)
	if len(options) == 0 {
		return ""
	}

	return fmt.Sprintf("For tool '%s', parameter '%s', available options: %s", 
		toolName, fieldKey, strings.Join(options, ", "))
}

func (f *FunctionCallingConversationManager) extractOptions(peekResults []domain.PeekResultItem) []string {
	var options []string
	
	for _, item := range peekResults {
		if item.Content != "" {
			options = append(options, fmt.Sprintf(`"%s"`, item.Content))
		}
	}
	
	return options
}

func (f *FunctionCallingConversationManager) formatToolDescriptions() string {
	if len(f.toolDefinitions) == 0 {
		return "No tools available"
	}

	descriptions := make([]string, 0, len(f.toolDefinitions))
	for _, tool := range f.toolDefinitions {
		toolDesc := fmt.Sprintf("- %s: %s", tool.Name, tool.Description)

		// Add information about pre-configured parameters if any
		if len(tool.IntegrationSettings) > 0 {
			preConfigured := []string{}
			for key, value := range tool.IntegrationSettings {
				if value != "" && value != nil {
					preConfigured = append(preConfigured, fmt.Sprintf("%s=%v", key, value))
				}
			}
			if len(preConfigured) > 0 {
				toolDesc += fmt.Sprintf("\n  Pre-configured: %s", strings.Join(preConfigured, ", "))
			}
		}

		descriptions = append(descriptions, toolDesc)
	}

	result := ""
	for _, desc := range descriptions {
		result += desc + "\n"
	}
	return result
}

func (f *FunctionCallingConversationManager) formatToolResult(result *ToolCallResult) string {
	if !result.Success {
		return fmt.Sprintf("Error: %s", result.Error)
	}

	if resultStr, ok := result.Result.(string); ok {
		return resultStr
	}

	// Convert complex result to JSON string
	if jsonBytes, err := json.Marshal(result.Result); err == nil {
		return string(jsonBytes)
	}

	return fmt.Sprintf("%+v", result.Result)
}

func (f *FunctionCallingConversationManager) getToolNames() []string {
	var toolNames []string
	for _, toolDef := range f.toolDefinitions {
		toolNames = append(toolNames, toolDef.Name)
	}
	return toolNames
}

const maxFCToolFailures = 3

func (f *FunctionCallingConversationManager) shouldContinueAfterError(state *FunctionCallingState, _ error) bool {
	return state.ToolFailures < maxFCToolFailures
}

func (f *FunctionCallingConversationManager) buildConversationResult(state *FunctionCallingState) *ConversationResult {
	finalResponse := "Task completed"

	// Get the last assistant message content
	for i := len(state.ConversationHistory) - 1; i >= 0; i-- {
		if state.ConversationHistory[i].Role == "assistant" && state.ConversationHistory[i].Content != "" {
			finalResponse = state.ConversationHistory[i].Content
			break
		}
	}

	// If still no response, provide a summary based on tool executions
	if finalResponse == "Task completed" && len(state.ToolExecutions) > 0 {
		successCount := 0
		for _, exec := range state.ToolExecutions {
			if exec.Success {
				successCount++
			}
		}
		finalResponse = fmt.Sprintf("Executed %d tools successfully (%d total executions)",
			successCount, len(state.ToolExecutions))
	}

	log.Debug().
		Str("conversation_id", state.ConversationID).
		Str("final_response", finalResponse).
		Int("conversation_length", len(state.ConversationHistory)).
		Int("tool_executions", len(state.ToolExecutions)).
		Msg("Building conversation result")

	// Convert tool executions to interface slice
	toolExecutions := make([]interface{}, len(state.ToolExecutions))
	for i, exec := range state.ToolExecutions {
		toolExecutions[i] = exec
	}

	return &ConversationResult{
		FinalResponse:  finalResponse,
		ToolExecutions: toolExecutions,
	}
}

func (f *FunctionCallingConversationManager) saveStateWithRetry(ctx context.Context, state *FunctionCallingState) {
	for i := 0; i < 3; i++ {
		if err := f.stateManager.SaveState(ctx, state); err == nil {
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
	log.Error().Str("conversation_id", state.ConversationID).Msg("Failed to save state after retries")
}

// generateWithConversationAndEvents wraps LLM calls with event publishing
func (f *FunctionCallingConversationManager) generateWithConversationAndEvents(
	ctx context.Context,
	req domain.GenerateRequest,
) (domain.ModelResponse, error) {
	// Get workflow execution context for event publishing
	workflowCtx, hasWorkflowCtx := domain.GetWorkflowExecutionContext(ctx)

	// Publish LLM execution started event
	if hasWorkflowCtx && workflowCtx.EnableEvents {
		err := f.publishLLMExecutionStartedEvent(ctx, workflowCtx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish LLM execution started event")
		}
	}

	// Call the LLM
	response, err := f.llm.GenerateWithConversation(ctx, req)

	if err != nil {
		// Publish LLM failed event
		if hasWorkflowCtx && workflowCtx.EnableEvents {
			publishErr := f.publishLLMExecutionFailedEvent(ctx, workflowCtx, req, err)
			if publishErr != nil {
				log.Error().Err(publishErr).Msg("Failed to publish LLM execution failed event")
			}
		}
		return domain.ModelResponse{}, err
	}

	// Publish LLM execution completed event
	if hasWorkflowCtx && workflowCtx.EnableEvents {
		err = f.publishLLMExecutionCompletedEvent(ctx, workflowCtx, req, &response)
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish LLM execution completed event")
		}
	}

	return response, nil
}

// publishLLMExecutionStartedEvent publishes a NodeExecutionStartedEvent for LLM call
func (f *FunctionCallingConversationManager) publishLLMExecutionStartedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
) error {
	event := &domain.NodeExecutionStartedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              f.executeParams.LLM.NodeID,
		Timestamp:           time.Now().UnixNano(),
	}

	return f.eventPublisher.PublishEvent(ctx, event)
}

// publishLLMExecutionCompletedEvent publishes a NodeExecutedEvent for successful LLM call
func (f *FunctionCallingConversationManager) publishLLMExecutionCompletedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	req domain.GenerateRequest,
	response *domain.ModelResponse,
) error {
	// Build input items from LLM request
	inputItems := f.buildInputItemsFromLLMRequest(req)

	// Build output items from LLM response
	outputItems := f.buildOutputItemsFromLLMResponse(response, f.executeParams.LLM.NodeID)

	event := &domain.NodeExecutedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              f.executeParams.LLM.NodeID,
		ItemsByInputID:      inputItems,
		ItemsByOutputID:     outputItems,
		Timestamp:           time.Now().UnixNano(),
		ExecutionOrder:      0, // OrderedEventPublisher will handle ordering via EventOrder field
	}

	return f.eventPublisher.PublishEvent(ctx, event)
}

// publishLLMExecutionFailedEvent publishes a NodeFailedEvent for failed LLM call
func (f *FunctionCallingConversationManager) publishLLMExecutionFailedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	req domain.GenerateRequest,
	err error,
) error {
	// Build input items from LLM request
	inputItems := f.buildInputItemsFromLLMRequest(req)

	event := &domain.NodeFailedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              f.executeParams.LLM.NodeID,
		Error:               err.Error(),
		ExecutionOrder:      0, // OrderedEventPublisher will handle ordering via EventOrder field
		Timestamp:           time.Now().UnixNano(),
		ItemsByInputID:      inputItems,
		ItemsByOutputID:     make(map[string]domain.NodeItems), // No output on failure
	}

	return f.eventPublisher.PublishEvent(ctx, event)
}

// buildInputItemsFromLLMRequest converts LLM request to NodeItems format
func (f *FunctionCallingConversationManager) buildInputItemsFromLLMRequest(req domain.GenerateRequest) map[string]domain.NodeItems {
	// Create item containing the request details
	requestItem := map[string]interface{}{
		"messages":      req.Messages,
		"system_prompt": req.SystemPrompt,
		"temperature":   req.Temperature,
		"max_tokens":    req.MaxTokens,
		"model":         req.Model,
		"tool_count":    len(req.Tools),
	}

	items := []domain.Item{domain.Item(requestItem)}

	inputID := "llm_input"
	return map[string]domain.NodeItems{
		inputID: {
			FromNodeID: f.agentNodeID, // LLM calls originate from AI agent
			Items:      items,
		},
	}
}

// buildOutputItemsFromLLMResponse converts LLM response to NodeItems format
func (f *FunctionCallingConversationManager) buildOutputItemsFromLLMResponse(response *domain.ModelResponse, fromNodeID string) map[string]domain.NodeItems {
	// Create item containing the response details
	responseItem := map[string]interface{}{
		"content":       response.Content,
		"tool_calls":    len(response.ToolCalls),
		"finish_reason": response.FinishReason,
	}

	items := []domain.Item{domain.Item(responseItem)}

	outputID := fmt.Sprintf("output-%s-0", fromNodeID)
	return map[string]domain.NodeItems{
		outputID: {
			FromNodeID: fromNodeID,
			Items:      items,
		},
	}
}

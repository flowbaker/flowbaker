package ai_agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/internal/domain"

	"github.com/rs/xid"
)

// ConversationRecordBuilder handles conversion of conversation state to AgentConversation records
type ConversationRecordBuilder interface {
	// BuildFromState creates an AgentConversation from FunctionCallingState
	BuildFromState(state *FunctionCallingState, status string) *domain.AgentConversation

	// BuildFromStateAndResult creates an AgentConversation from state and final result
	BuildFromStateAndResult(
		state *FunctionCallingState,
		result *ConversationResult,
		workspaceID,
		initialPrompt,
		status string,
	) *domain.AgentConversation

	// UpdateRecordWithProgress updates an existing record with new state
	UpdateRecordWithProgress(record *domain.AgentConversation, state *FunctionCallingState) *domain.AgentConversation
}

// DefaultConversationRecordBuilder implements ConversationRecordBuilder
type DefaultConversationRecordBuilder struct {
	agentNodeID string
	config      ConversationMemoryConfig
}

// NewConversationRecordBuilder creates a new conversation record builder
func NewConversationRecordBuilder(agentNodeID string, config ConversationMemoryConfig) ConversationRecordBuilder {
	return &DefaultConversationRecordBuilder{
		agentNodeID: agentNodeID,
		config:      config,
	}
}

// BuildFromState creates an AgentConversation from FunctionCallingState
func (b *DefaultConversationRecordBuilder) BuildFromState(state *FunctionCallingState, status string) *domain.AgentConversation {
	if state == nil {
		return nil
	}

	return &domain.AgentConversation{
		ID:             xid.New().String(),
		WorkspaceID:    state.WorkspaceID,
		SessionID:      b.config.SessionID,
		ConversationID: state.ConversationID,
		Messages:       b.convertConversationMessages(state.ConversationHistory),
		CreatedAt:      state.CreatedAt,
		UpdatedAt:      state.UpdatedAt,
		UserPrompt:     b.extractUserPrompt(state.ConversationHistory),
		FinalResponse:  b.extractFinalResponse(state.ConversationHistory),
		ToolsUsed:      b.extractToolsUsed(state.ToolExecutions),
		Status:         status,
		Metadata:       b.buildMetadata(state),
	}
}

// BuildFromStateAndResult creates an AgentConversation from state and final result
func (b *DefaultConversationRecordBuilder) BuildFromStateAndResult(
	state *FunctionCallingState,
	result *ConversationResult,
	workspaceID,
	initialPrompt,
	status string,
) *domain.AgentConversation {
	if state == nil {
		return nil
	}

	record := b.BuildFromState(state, status)
	if record == nil {
		return nil
	}

	// Override with provided values
	if workspaceID != "" {
		record.WorkspaceID = workspaceID
	}

	if initialPrompt != "" {
		record.UserPrompt = initialPrompt
	}

	// Use result for final response if available
	if result != nil && result.FinalResponse != "" {
		record.FinalResponse = result.FinalResponse

		// Add result metadata
		if record.Metadata == nil {
			record.Metadata = make(map[string]interface{})
		}
		record.Metadata["result_tool_executions"] = len(result.ToolExecutions)
	}

	return record
}

// UpdateRecordWithProgress updates an existing record with new state
func (b *DefaultConversationRecordBuilder) UpdateRecordWithProgress(
	record *domain.AgentConversation,
	state *FunctionCallingState,
) *domain.AgentConversation {
	if record == nil || state == nil {
		return record
	}

	// Update mutable fields
	record.Messages = b.convertConversationMessages(state.ConversationHistory)
	record.UpdatedAt = state.UpdatedAt
	record.FinalResponse = b.extractFinalResponse(state.ConversationHistory)
	record.ToolsUsed = b.extractToolsUsed(state.ToolExecutions)
	record.Metadata = b.buildMetadata(state)

	return record
}

// convertConversationMessages converts domain.ConversationMessage to domain.AgentMessage
func (b *DefaultConversationRecordBuilder) convertConversationMessages(
	conversations []domain.ConversationMessage,
) []domain.AgentMessage {
	if len(conversations) == 0 {
		return []domain.AgentMessage{}
	}

	agentMessages := make([]domain.AgentMessage, 0, len(conversations))

	for _, conv := range conversations {
		agentMessage := domain.AgentMessage{
			Role:      conv.Role,
			Content:   conv.Content,
			Timestamp: time.Now(), // ConversationMessage doesn't have timestamp
		}

		// Convert tool calls
		if len(conv.ToolCalls) > 0 {
			agentToolCalls := make([]domain.AgentToolCall, 0, len(conv.ToolCalls))
			for _, tc := range conv.ToolCalls {
				agentToolCalls = append(agentToolCalls, domain.AgentToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
			}
			agentMessage.ToolCalls = agentToolCalls
		}

		// Convert tool results
		if len(conv.ToolResults) > 0 {
			agentToolResults := make([]domain.AgentToolResult, 0, len(conv.ToolResults))
			for _, tr := range conv.ToolResults {
				agentToolResults = append(agentToolResults, domain.AgentToolResult{
					ToolCallID: tr.ToolCallID,
					Content:    tr.Content,
				})
			}
			agentMessage.ToolResults = agentToolResults
		}

		agentMessages = append(agentMessages, agentMessage)
	}

	return agentMessages
}

// extractUserPrompt extracts the initial user prompt from conversation history
func (b *DefaultConversationRecordBuilder) extractUserPrompt(conversations []domain.ConversationMessage) string {
	for _, conv := range conversations {
		if conv.Role == "user" && strings.TrimSpace(conv.Content) != "" {
			return conv.Content
		}
	}
	return ""
}

// extractFinalResponse extracts the final assistant response from conversation history
func (b *DefaultConversationRecordBuilder) extractFinalResponse(conversations []domain.ConversationMessage) string {
	// Look for the last assistant message with content
	for i := len(conversations) - 1; i >= 0; i-- {
		message := conversations[i]
		if message.Role == "assistant" && strings.TrimSpace(message.Content) != "" {
			return message.Content
		}
	}

	return ""
}

// extractToolsUsed extracts unique tool names from executions
func (b *DefaultConversationRecordBuilder) extractToolsUsed(executions []FunctionCallExecution) []string {
	if len(executions) == 0 {
		return []string{}
	}

	toolSet := make(map[string]bool)
	for _, exec := range executions {
		if exec.ToolName != "" {
			toolSet[exec.ToolName] = true
		}
	}

	tools := make([]string, 0, len(toolSet))
	for tool := range toolSet {
		tools = append(tools, tool)
	}

	return tools
}

// buildMetadata creates metadata from state
func (b *DefaultConversationRecordBuilder) buildMetadata(state *FunctionCallingState) map[string]interface{} {
	metadata := map[string]interface{}{
		"agent_node_id":    b.agentNodeID,
		"rounds":           state.Round,
		"tool_failures":    state.ToolFailures,
		"current_step":     string(state.CurrentStep),
		"execution_status": string(state.Status),
	}

	// Add execution time if available
	if !state.CreatedAt.IsZero() && !state.UpdatedAt.IsZero() {
		metadata["execution_time"] = state.UpdatedAt.Sub(state.CreatedAt).Seconds()
	}

	// Add error information if available
	if state.LastError != nil {
		metadata["last_error"] = map[string]interface{}{
			"type":        state.LastError.Type,
			"message":     state.LastError.Message,
			"round":       state.LastError.Round,
			"step":        string(state.LastError.Step),
			"recoverable": state.LastError.Recoverable,
			"timestamp":   state.LastError.Timestamp,
		}
	}

	// Add tool execution statistics
	if len(state.ToolExecutions) > 0 {
		successCount := 0
		totalDuration := time.Duration(0)

		for _, exec := range state.ToolExecutions {
			if exec.Success {
				successCount++
			}
			totalDuration += exec.Duration
		}

		metadata["tool_executions"] = map[string]interface{}{
			"total":            len(state.ToolExecutions),
			"successful":       successCount,
			"failed":           len(state.ToolExecutions) - successCount,
			"total_duration":   totalDuration.Seconds(),
			"average_duration": totalDuration.Seconds() / float64(len(state.ToolExecutions)),
		}
	}

	// Add context information if available
	if len(state.Context) > 0 {
		metadata["context_keys"] = b.getContextKeys(state.Context)
	}

	return metadata
}

// getContextKeys extracts keys from context map for metadata
func (b *DefaultConversationRecordBuilder) getContextKeys(context map[string]interface{}) []string {
	keys := make([]string, 0, len(context))
	for key := range context {
		keys = append(keys, key)
	}
	return keys
}

// BuildConversationSummary creates a summary string for the conversation
func (b *DefaultConversationRecordBuilder) BuildConversationSummary(record *domain.AgentConversation) string {
	if record == nil {
		return ""
	}

	summary := fmt.Sprintf("Conversation %s (%s)",
		record.ConversationID,
		record.CreatedAt.Format("2006-01-02 15:04"))

	if record.UserPrompt != "" {
		prompt := record.UserPrompt
		if len(prompt) > 100 {
			prompt = prompt[:100] + "..."
		}
		summary += fmt.Sprintf("\nPrompt: %s", prompt)
	}

	if len(record.ToolsUsed) > 0 {
		summary += fmt.Sprintf("\nTools: %s", strings.Join(record.ToolsUsed, ", "))
	}

	if record.Status != "" {
		summary += fmt.Sprintf("\nStatus: %s", record.Status)
	}

	return summary
}

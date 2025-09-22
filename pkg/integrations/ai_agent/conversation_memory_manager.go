package ai_agent

import (
	"context"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
)

// ConversationMemoryManager handles memory operations for AI agent conversations
type ConversationMemoryManager interface {
	// RetrieveMemoryContext gets relevant past conversations and formats them as context
	RetrieveMemoryContext(ctx context.Context, workspaceID string) (string, error)

	// StoreConversationStart stores initial conversation metadata
	StoreConversationStart(ctx context.Context, conversationID, workspaceID, initialPrompt string) error

	// StoreConversationComplete stores final conversation result
	StoreConversationComplete(ctx context.Context, state *FunctionCallingState, result *ConversationResult, workspaceID, initialPrompt string) error

	// EnhanceSystemPrompt adds memory context to system prompt
	EnhanceSystemPrompt(ctx context.Context, basePrompt, workspaceID string) (string, error)
}

// ConversationMemoryConfig contains memory configuration
type ConversationMemoryConfig struct {
	Enabled           bool   `json:"enabled"`
	SessionID         string `json:"session_id"`
	ConversationCount int    `json:"conversation_count"`
	IncludeToolUsage  bool   `json:"include_tool_usage"`
	MaxContextLength  int    `json:"max_context_length"`
}

// DefaultConversationMemoryManager implements ConversationMemoryManager
type DefaultConversationMemoryManager struct {
	memory         domain.IntegrationMemory
	contextBuilder MemoryContextBuilder
	recordBuilder  ConversationRecordBuilder
	config         ConversationMemoryConfig
	agentNodeID    string
	eventPublisher domain.EventPublisher
	memoryNodeID   string
}

// ConversationMemoryManagerDependencies contains dependencies for memory manager
type ConversationMemoryManagerDependencies struct {
	Memory         domain.IntegrationMemory
	AgentNodeID    string
	Config         ConversationMemoryConfig
	EventPublisher domain.EventPublisher
	MemoryNodeID   string
}

// NewConversationMemoryManager creates a new conversation memory manager
func NewConversationMemoryManager(deps ConversationMemoryManagerDependencies) ConversationMemoryManager {
	if deps.Memory == nil {
		return &NoOpConversationMemoryManager{}
	}

	return &DefaultConversationMemoryManager{
		memory:         deps.Memory,
		contextBuilder: NewMemoryContextBuilder(deps.Config),
		recordBuilder:  NewConversationRecordBuilder(deps.AgentNodeID, deps.Config),
		config:         deps.Config,
		agentNodeID:    deps.AgentNodeID,
		eventPublisher: deps.EventPublisher,
		memoryNodeID:   deps.MemoryNodeID,
	}
}

// RetrieveMemoryContext retrieves and formats memory context
func (m *DefaultConversationMemoryManager) RetrieveMemoryContext(ctx context.Context, workspaceID string) (string, error) {
	if !m.config.Enabled {
		return "", nil
	}

	startTime := time.Now()

	// Get workflow execution context for event publishing
	workflowCtx, hasWorkflowCtx := domain.GetWorkflowExecutionContext(ctx)

	// Publish memory retrieval started event
	if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
		err := m.publishMemoryRetrievalStartedEvent(ctx, workflowCtx, workspaceID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish memory retrieval started event")
		}
	}

	filter := domain.ConversationFilter{
		SessionID:    m.config.SessionID,
		WorkspaceID:  workspaceID,
		Limit:        m.config.ConversationCount,
		Status:       "completed",
		IncludeTools: m.config.IncludeToolUsage,
	}

	log.Debug().
		Str("agent_node_id", m.agentNodeID).
		Str("workspace_id", workspaceID).
		Interface("filter", filter).
		Msg("Retrieving memory context")

	conversations, err := m.memory.RetrieveConversations(ctx, filter)
	if err != nil {
		log.Debug().
			Err(err).
			Msg("No previous memory found, starting fresh conversation")

		if hasWorkflowCtx && m.memoryNodeID != "" && m.config.Enabled {
			inputItems := m.buildMemoryInputItems(filter)
			agentExecution := domain.NodeExecutionEntry{
				NodeID:          m.memoryNodeID,
				Error:           err.Error(),
				ItemsByInputID:  inputItems,
				ItemsByOutputID: make(map[string]domain.NodeItems),
				EventType:       domain.NodeFailed,
				Timestamp:       startTime.UnixNano(),
				ExecutionOrder:  1,
			}
			workflowCtx.AddAgentNodeExecution(agentExecution)

			if workflowCtx.EnableEvents && m.eventPublisher != nil {
				publishErr := m.publishMemoryRetrievalFailedEvent(ctx, workflowCtx, workspaceID, err)
				if publishErr != nil {
					log.Error().Err(publishErr).Msg("Failed to publish memory retrieval failed event")
				}
			}
		}

		return "", nil
	}

	if len(conversations) == 0 {
		if hasWorkflowCtx && m.memoryNodeID != "" && m.config.Enabled {
			inputItems := m.buildMemoryInputItems(filter)
			outputItems := m.buildMemoryOutputItems([]domain.AgentConversation{}, m.memoryNodeID)
			agentExecution := domain.NodeExecutionEntry{
				NodeID:          m.memoryNodeID,
				Error:           "",
				ItemsByInputID:  inputItems,
				ItemsByOutputID: outputItems,
				EventType:       domain.NodeExecuted,
				Timestamp:       startTime.UnixNano(),
				ExecutionOrder:  1,
			}
			workflowCtx.AddAgentNodeExecution(agentExecution)

			if workflowCtx.EnableEvents && m.eventPublisher != nil {
				publishErr := m.publishMemoryRetrievalCompletedEvent(ctx, workflowCtx, workspaceID, 0, 0, startTime)
				if publishErr != nil {
					log.Error().Err(publishErr).Msg("Failed to publish memory retrieval completed event")
				}
			}
		}
		return "", nil
	}

	context := m.contextBuilder.FormatMultipleConversations(conversations)

	if hasWorkflowCtx && m.memoryNodeID != "" && m.config.Enabled {
		inputItems := m.buildMemoryInputItems(filter)
		outputItems := m.buildMemoryOutputItems(conversations, m.memoryNodeID)
		agentExecution := domain.NodeExecutionEntry{
			NodeID:          m.memoryNodeID,
			Error:           "",
			ItemsByInputID:  inputItems,
			ItemsByOutputID: outputItems,
			EventType:       domain.NodeExecuted,
			Timestamp:       startTime.UnixNano(),
			ExecutionOrder:  1,
		}
		workflowCtx.AddAgentNodeExecution(agentExecution)

		if workflowCtx.EnableEvents && m.eventPublisher != nil {
			err = m.publishMemoryRetrievalCompletedEvent(ctx, workflowCtx, workspaceID, len(conversations), len(context), startTime)
			if err != nil {
				log.Error().Err(err).Msg("Failed to publish memory retrieval completed event")
			}
		}
	}

	log.Debug().
		Str("agent_node_id", m.agentNodeID).
		Int("context_length", len(context)).
		Int("conversation_count", len(conversations)).
		Msg("Retrieved memory context")

	return context, nil
}

// StoreConversationStart stores initial conversation metadata
func (m *DefaultConversationMemoryManager) StoreConversationStart(ctx context.Context, conversationID, workspaceID, initialPrompt string) error {
	if !m.config.Enabled {
		return nil
	}

	startTime := time.Now()

	// Get workflow execution context for event publishing
	workflowCtx, hasWorkflowCtx := domain.GetWorkflowExecutionContext(ctx)

	// Publish memory storage started event
	if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
		err := m.publishMemoryStorageStartedEvent(ctx, workflowCtx, conversationID, "start")
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish memory storage started event")
		}
	}

	record := &domain.AgentConversation{
		ID:             xid.New().String(),
		WorkspaceID:    workspaceID,
		SessionID:      m.config.SessionID,
		ConversationID: conversationID,
		UserPrompt:     initialPrompt,
		Status:         "started",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Messages:       []domain.AgentMessage{},
		ToolsUsed:      []string{},
		Metadata: map[string]interface{}{
			"agent_node_id": m.agentNodeID,
			"phase":         "initialization",
		},
	}

	err := m.storeConversationRecord(ctx, record, "start")

	if err != nil {
		// Publish memory storage failed event
		if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
			publishErr := m.publishMemoryStorageFailedEvent(ctx, workflowCtx, conversationID, "start", err)
			if publishErr != nil {
				log.Error().Err(publishErr).Msg("Failed to publish memory storage failed event")
			}
		}
		return err
	}

	// Publish memory storage completed event
	if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
		publishErr := m.publishMemoryStorageCompletedEvent(ctx, workflowCtx, conversationID, "start", record.ID, startTime)
		if publishErr != nil {
			log.Error().Err(publishErr).Msg("Failed to publish memory storage completed event")
		}
	}

	return nil
}

// StoreConversationComplete stores final conversation result
func (m *DefaultConversationMemoryManager) StoreConversationComplete(
	ctx context.Context,
	state *FunctionCallingState,
	result *ConversationResult,
	workspaceID,
	initialPrompt string,
) error {
	if !m.config.Enabled {
		return nil
	}

	startTime := time.Now()

	// Get workflow execution context for event publishing
	workflowCtx, hasWorkflowCtx := domain.GetWorkflowExecutionContext(ctx)

	// Publish memory storage started event
	if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
		err := m.publishMemoryStorageStartedEvent(ctx, workflowCtx, state.ConversationID, "complete")
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish memory storage started event")
		}
	}

	record := m.recordBuilder.BuildFromStateAndResult(state, result, workspaceID, initialPrompt, "completed")
	err := m.storeConversationRecord(ctx, record, "complete")

	if err != nil {
		// Publish memory storage failed event
		if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
			publishErr := m.publishMemoryStorageFailedEvent(ctx, workflowCtx, state.ConversationID, "complete", err)
			if publishErr != nil {
				log.Error().Err(publishErr).Msg("Failed to publish memory storage failed event")
			}
		}
		return err
	}

	// Publish memory storage completed event
	if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
		publishErr := m.publishMemoryStorageCompletedEvent(ctx, workflowCtx, state.ConversationID, "complete", record.ID, startTime)
		if publishErr != nil {
			log.Error().Err(publishErr).Msg("Failed to publish memory storage completed event")
		}
	}

	return nil
}

// EnhanceSystemPrompt adds memory context to system prompt
func (m *DefaultConversationMemoryManager) EnhanceSystemPrompt(ctx context.Context, basePrompt, workspaceID string) (string, error) {
	if !m.config.Enabled {
		return basePrompt, nil
	}

	startTime := time.Now()

	// Get workflow execution context for event publishing
	workflowCtx, hasWorkflowCtx := domain.GetWorkflowExecutionContext(ctx)

	// Publish memory enhancement started event
	if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
		err := m.publishMemoryEnhancementStartedEvent(ctx, workflowCtx, workspaceID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish memory enhancement started event")
		}
	}

	memoryContext, err := m.RetrieveMemoryContext(ctx, workspaceID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to retrieve memory context for system prompt")

		// Publish memory enhancement failed event
		if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
			publishErr := m.publishMemoryEnhancementFailedEvent(ctx, workflowCtx, workspaceID, err)
			if publishErr != nil {
				log.Error().Err(publishErr).Msg("Failed to publish memory enhancement failed event")
			}
		}

		return basePrompt, nil // Don't fail the conversation due to memory issues
	}

	if memoryContext == "" {
		// Publish memory enhancement completed event with no context
		if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
			publishErr := m.publishMemoryEnhancementCompletedEvent(ctx, workflowCtx, workspaceID, len(basePrompt), len(basePrompt), 0, startTime)
			if publishErr != nil {
				log.Error().Err(publishErr).Msg("Failed to publish memory enhancement completed event")
			}
		}
		return basePrompt, nil
	}

	enhanced := m.contextBuilder.EnhanceSystemPrompt(basePrompt, memoryContext)

	// Publish memory enhancement completed event
	if hasWorkflowCtx && workflowCtx.EnableEvents && m.eventPublisher != nil && m.memoryNodeID != "" {
		publishErr := m.publishMemoryEnhancementCompletedEvent(ctx, workflowCtx, workspaceID, len(basePrompt), len(enhanced), len(memoryContext), startTime)
		if publishErr != nil {
			log.Error().Err(publishErr).Msg("Failed to publish memory enhancement completed event")
		}
	}

	log.Debug().
		Int("base_prompt_length", len(basePrompt)).
		Int("enhanced_prompt_length", len(enhanced)).
		Int("memory_context_length", len(memoryContext)).
		Msg("Enhanced system prompt with memory context")

	return enhanced, nil
}

// storeConversationRecord stores a conversation record in memory
func (m *DefaultConversationMemoryManager) storeConversationRecord(ctx context.Context, record *domain.AgentConversation, phase string) error {
	request := domain.StoreConversationRequest{
		Conversation: record,
		PartitionKey: record.WorkspaceID,
		TTL:          0, // No expiry by default, can be configured later
	}

	err := m.memory.StoreConversation(ctx, request)
	if err != nil {
		log.Error().
			Err(err).
			Str("conversation_id", record.ConversationID).
			Str("phase", phase).
			Msg("Failed to store conversation record")
		return err
	}

	log.Debug().
		Str("conversation_id", record.ConversationID).
		Str("phase", phase).
		Str("status", record.Status).
		Int("message_count", len(record.Messages)).
		Int("tools_used", len(record.ToolsUsed)).
		Msg("Stored conversation record in memory")

	return nil
}

// NoOpConversationMemoryManager is a no-op implementation for when memory is disabled
type NoOpConversationMemoryManager struct{}

func (m *NoOpConversationMemoryManager) RetrieveMemoryContext(ctx context.Context, workspaceID string) (string, error) {
	return "", nil
}

func (m *NoOpConversationMemoryManager) StoreConversationStart(ctx context.Context, conversationID, workspaceID, initialPrompt string) error {
	return nil
}

func (m *NoOpConversationMemoryManager) StoreConversationComplete(
	ctx context.Context,
	state *FunctionCallingState,
	result *ConversationResult,
	workspaceID,
	initialPrompt string,
) error {
	return nil
}

func (m *NoOpConversationMemoryManager) EnhanceSystemPrompt(ctx context.Context, basePrompt, workspaceID string) (string, error) {
	return basePrompt, nil
}

// publishMemoryRetrievalStartedEvent publishes a NodeExecutionStartedEvent for memory retrieval
func (m *DefaultConversationMemoryManager) publishMemoryRetrievalStartedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	workspaceID string,
) error {
	event := &domain.NodeExecutionStartedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              m.memoryNodeID,
		Timestamp:           time.Now().UnixNano(),
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// publishMemoryRetrievalCompletedEvent publishes a NodeExecutedEvent for successful memory retrieval
func (m *DefaultConversationMemoryManager) publishMemoryRetrievalCompletedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	workspaceID string,
	conversationsFound int,
	contextLength int,
	startTime time.Time,
) error {
	duration := time.Since(startTime).Milliseconds()

	// Build input items from retrieval parameters
	inputItems := m.buildMemoryRetrievalInputItems(workspaceID)

	// Build output items from retrieval results
	outputItems := m.buildMemoryRetrievalOutputItems(conversationsFound, contextLength, duration)

	event := &domain.NodeExecutedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              m.memoryNodeID,
		ItemsByInputID:      inputItems,
		ItemsByOutputID:     outputItems,
		Timestamp:           time.Now().UnixNano(),
		ExecutionOrder:      0, // OrderedEventPublisher will handle ordering via EventOrder field
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// publishMemoryRetrievalFailedEvent publishes a NodeFailedEvent for failed memory retrieval
func (m *DefaultConversationMemoryManager) publishMemoryRetrievalFailedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	workspaceID string,
	err error,
) error {
	// Build input items from retrieval parameters
	inputItems := m.buildMemoryRetrievalInputItems(workspaceID)

	event := &domain.NodeFailedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              m.memoryNodeID,
		Error:               err.Error(),
		ExecutionOrder:      0, // OrderedEventPublisher will handle ordering via EventOrder field
		Timestamp:           time.Now().UnixNano(),
		ItemsByInputID:      inputItems,
		ItemsByOutputID:     make(map[string]domain.NodeItems), // No output on failure
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// buildMemoryRetrievalInputItems builds input items for memory retrieval events
func (m *DefaultConversationMemoryManager) buildMemoryRetrievalInputItems(workspaceID string) map[string]domain.NodeItems {
	inputItem := map[string]interface{}{
		"operation":          "memory_retrieval",
		"session_id":         m.config.SessionID,
		"workspace_id":       workspaceID,
		"conversation_count": m.config.ConversationCount,
		"include_tools":      m.config.IncludeToolUsage,
		"max_context_length": m.config.MaxContextLength,
	}

	items := []domain.Item{domain.Item(inputItem)}

	inputID := "memory_input"
	return map[string]domain.NodeItems{
		inputID: {
			FromNodeID: m.agentNodeID, // Memory operations originate from AI agent
			Items:      items,
		},
	}
}

// buildMemoryRetrievalOutputItems builds output items for memory retrieval events
func (m *DefaultConversationMemoryManager) buildMemoryRetrievalOutputItems(conversationsFound, contextLength int, durationMs int64) map[string]domain.NodeItems {
	outputItem := map[string]interface{}{
		"operation":             "memory_retrieval",
		"conversations_found":   conversationsFound,
		"context_length":        contextLength,
		"retrieval_duration_ms": durationMs,
		"success":               true,
	}

	items := []domain.Item{domain.Item(outputItem)}

	outputID := fmt.Sprintf("output-%s-0", m.memoryNodeID)
	return map[string]domain.NodeItems{
		outputID: {
			FromNodeID: m.memoryNodeID,
			Items:      items,
		},
	}
}

// publishMemoryStorageStartedEvent publishes a NodeExecutionStartedEvent for memory storage
func (m *DefaultConversationMemoryManager) publishMemoryStorageStartedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	conversationID string,
	phase string,
) error {
	event := &domain.NodeExecutionStartedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              m.memoryNodeID,
		Timestamp:           time.Now().UnixNano(),
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// publishMemoryStorageCompletedEvent publishes a NodeExecutedEvent for successful memory storage
func (m *DefaultConversationMemoryManager) publishMemoryStorageCompletedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	conversationID string,
	phase string,
	recordID string,
	startTime time.Time,
) error {
	duration := time.Since(startTime).Milliseconds()

	// Build input items from storage parameters
	inputItems := m.buildMemoryStorageInputItems(conversationID, phase)

	// Build output items from storage results
	outputItems := m.buildMemoryStorageOutputItems(recordID, phase, duration)

	event := &domain.NodeExecutedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              m.memoryNodeID,
		ItemsByInputID:      inputItems,
		ItemsByOutputID:     outputItems,
		Timestamp:           time.Now().UnixNano(),
		ExecutionOrder:      0, // OrderedEventPublisher will handle ordering via EventOrder field
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// publishMemoryStorageFailedEvent publishes a NodeFailedEvent for failed memory storage
func (m *DefaultConversationMemoryManager) publishMemoryStorageFailedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	conversationID string,
	phase string,
	err error,
) error {
	// Build input items from storage parameters
	inputItems := m.buildMemoryStorageInputItems(conversationID, phase)

	event := &domain.NodeFailedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              m.memoryNodeID,
		Error:               err.Error(),
		ExecutionOrder:      0, // OrderedEventPublisher will handle ordering via EventOrder field
		Timestamp:           time.Now().UnixNano(),
		ItemsByInputID:      inputItems,
		ItemsByOutputID:     make(map[string]domain.NodeItems), // No output on failure
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// publishMemoryEnhancementStartedEvent publishes a NodeExecutionStartedEvent for memory enhancement
func (m *DefaultConversationMemoryManager) publishMemoryEnhancementStartedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	workspaceID string,
) error {
	event := &domain.NodeExecutionStartedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              m.memoryNodeID,
		Timestamp:           time.Now().UnixNano(),
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// publishMemoryEnhancementCompletedEvent publishes a NodeExecutedEvent for successful memory enhancement
func (m *DefaultConversationMemoryManager) publishMemoryEnhancementCompletedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	workspaceID string,
	basePromptLength int,
	enhancedPromptLength int,
	contextLength int,
	startTime time.Time,
) error {
	duration := time.Since(startTime).Milliseconds()

	// Build input items from enhancement parameters
	inputItems := m.buildMemoryEnhancementInputItems(workspaceID, basePromptLength)

	// Build output items from enhancement results
	outputItems := m.buildMemoryEnhancementOutputItems(enhancedPromptLength, contextLength, duration)

	event := &domain.NodeExecutedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              m.memoryNodeID,
		ItemsByInputID:      inputItems,
		ItemsByOutputID:     outputItems,
		Timestamp:           time.Now().UnixNano(),
		ExecutionOrder:      0, // OrderedEventPublisher will handle ordering via EventOrder field
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// publishMemoryEnhancementFailedEvent publishes a NodeFailedEvent for failed memory enhancement
func (m *DefaultConversationMemoryManager) publishMemoryEnhancementFailedEvent(
	ctx context.Context,
	workflowCtx *domain.WorkflowExecutionContext,
	workspaceID string,
	err error,
) error {
	// Build input items from enhancement parameters
	inputItems := m.buildMemoryEnhancementInputItems(workspaceID, 0)

	event := &domain.NodeFailedEvent{
		WorkflowID:          workflowCtx.WorkflowID,
		WorkflowExecutionID: workflowCtx.WorkflowExecutionID,
		NodeID:              m.memoryNodeID,
		Error:               err.Error(),
		ExecutionOrder:      0, // OrderedEventPublisher will handle ordering via EventOrder field
		Timestamp:           time.Now().UnixNano(),
		ItemsByInputID:      inputItems,
		ItemsByOutputID:     make(map[string]domain.NodeItems), // No output on failure
	}

	return m.eventPublisher.PublishEvent(ctx, event)
}

// buildMemoryStorageInputItems builds input items for memory storage events
func (m *DefaultConversationMemoryManager) buildMemoryStorageInputItems(conversationID, phase string) map[string]domain.NodeItems {
	inputItem := map[string]interface{}{
		"operation":       "memory_storage",
		"conversation_id": conversationID,
		"session_id":      m.config.SessionID,
		"phase":           phase,
	}

	items := []domain.Item{domain.Item(inputItem)}

	inputID := "memory_input"
	return map[string]domain.NodeItems{
		inputID: {
			FromNodeID: m.agentNodeID, // Memory operations originate from AI agent
			Items:      items,
		},
	}
}

// buildMemoryStorageOutputItems builds output items for memory storage events
func (m *DefaultConversationMemoryManager) buildMemoryStorageOutputItems(recordID, phase string, durationMs int64) map[string]domain.NodeItems {
	outputItem := map[string]interface{}{
		"operation":           "memory_storage",
		"record_id":           recordID,
		"phase":               phase,
		"storage_duration_ms": durationMs,
		"success":             true,
	}

	items := []domain.Item{domain.Item(outputItem)}

	outputID := fmt.Sprintf("output-%s-0", m.memoryNodeID)
	return map[string]domain.NodeItems{
		outputID: {
			FromNodeID: m.memoryNodeID,
			Items:      items,
		},
	}
}

// buildMemoryEnhancementInputItems builds input items for memory enhancement events
func (m *DefaultConversationMemoryManager) buildMemoryEnhancementInputItems(workspaceID string, basePromptLength int) map[string]domain.NodeItems {
	inputItem := map[string]interface{}{
		"operation":          "memory_enhancement",
		"workspace_id":       workspaceID,
		"session_id":         m.config.SessionID,
		"base_prompt_length": basePromptLength,
	}

	items := []domain.Item{domain.Item(inputItem)}

	inputID := "memory_input"
	return map[string]domain.NodeItems{
		inputID: {
			FromNodeID: m.agentNodeID, // Memory operations originate from AI agent
			Items:      items,
		},
	}
}

// buildMemoryEnhancementOutputItems builds output items for memory enhancement events
func (m *DefaultConversationMemoryManager) buildMemoryEnhancementOutputItems(enhancedPromptLength, contextLength int, durationMs int64) map[string]domain.NodeItems {
	outputItem := map[string]interface{}{
		"operation":               "memory_enhancement",
		"enhanced_prompt_length":  enhancedPromptLength,
		"context_length":          contextLength,
		"enhancement_duration_ms": durationMs,
		"success":                 true,
	}

	items := []domain.Item{domain.Item(outputItem)}

	outputID := fmt.Sprintf("output-%s-0", m.memoryNodeID)
	return map[string]domain.NodeItems{
		outputID: {
			FromNodeID: m.memoryNodeID,
			Items:      items,
		},
	}
}

func (m *DefaultConversationMemoryManager) buildMemoryInputItems(filter domain.ConversationFilter) map[string]domain.NodeItems {
	inputItem := map[string]interface{}{
		"operation":    "memory_retrieval",
		"session_id":   filter.SessionID,
		"workspace_id": filter.WorkspaceID,
		"limit":        filter.Limit,
		"status":       filter.Status,
	}

	items := []domain.Item{domain.Item(inputItem)}
	inputID := fmt.Sprintf("input-%s-0", m.memoryNodeID)

	return map[string]domain.NodeItems{
		inputID: {
			FromNodeID: m.agentNodeID,
			Items:      items,
		},
	}
}

func (m *DefaultConversationMemoryManager) buildMemoryOutputItems(conversations []domain.AgentConversation, nodeID string) map[string]domain.NodeItems {
	outputItem := map[string]interface{}{
		"conversation_count": len(conversations),
		"status":             "retrieved",
	}

	items := []domain.Item{domain.Item(outputItem)}
	outputID := fmt.Sprintf("output-%s-0", nodeID)

	return map[string]domain.NodeItems{
		outputID: {
			FromNodeID: nodeID,
			Items:      items,
		},
	}
}

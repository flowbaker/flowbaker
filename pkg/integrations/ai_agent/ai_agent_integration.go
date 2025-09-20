package ai_agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
)

type ConversationResult struct {
	FinalResponse  string        `json:"final_response"`
	ToolExecutions []interface{} `json:"tool_executions,omitempty"`
}

const (
	IntegrationActionType_FunctionCallingAgent domain.IntegrationActionType = "function_calling_agent"
)

type AIAgentCreator struct {
	integrationSelector        domain.IntegrationSelector
	parameterBinder            domain.IntegrationParameterBinder
	executorIntegrationManager domain.ExecutorIntegrationManager
	eventPublisher             domain.EventPublisher
}

func NewAIAgentCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &AIAgentCreator{
		integrationSelector:        deps.IntegrationSelector,
		parameterBinder:            deps.ParameterBinder,
		executorIntegrationManager: deps.ExecutorIntegrationManager,
		eventPublisher:             deps.ExecutorEventPublisher,
	}
}

func (c *AIAgentCreator) CreateIntegration(ctx context.Context, params domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewAIAgentExecutorV2(domain.IntegrationDeps{
		IntegrationSelector:        c.integrationSelector,
		ParameterBinder:            c.parameterBinder,
		ExecutorIntegrationManager: c.executorIntegrationManager,
		ExecutorEventPublisher:     c.eventPublisher,
	}), nil
}

type AIAgentExecutorV2 struct {
	integrationSelector        domain.IntegrationSelector
	parameterBinder            domain.IntegrationParameterBinder
	executorIntegrationManager domain.ExecutorIntegrationManager
	eventPublisher             domain.EventPublisher
	actionManager              *domain.IntegrationActionManager
}

func NewAIAgentExecutorV2(deps domain.IntegrationDeps) domain.IntegrationExecutor {
	executor := &AIAgentExecutorV2{
		integrationSelector:        deps.IntegrationSelector,
		parameterBinder:            deps.ParameterBinder,
		executorIntegrationManager: deps.ExecutorIntegrationManager,
		eventPublisher:             deps.ExecutorEventPublisher,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItemMulti(IntegrationActionType_FunctionCallingAgent, executor.ProcessFunctionCalling)

	executor.actionManager = actionManager

	return executor
}

type NodeReference struct {
	NodeID string `json:"node_id"`
}

type AgentType string

const (
	AgentTypeReAct           AgentType = "react"
	AgentTypeFunctionCalling AgentType = "function_calling"
)

type ExecuteParams struct {
	Prompt string          `json:"prompt,omitempty"`
	LLM    *NodeReference  `json:"llm,omitempty"`
	Memory *NodeReference  `json:"memory,omitempty"`
	Tools  []NodeReference `json:"tools,omitempty"`
}

type MemoryNodeParams struct {
	SessionID           string `json:"session_id"`
	SessionTTLInSeconds int    `json:"session_ttl_in_seconds"`
	MaxContextLength    int    `json:"max_context_length"`
	ConversationCount   int    `json:"conversation_count"`
}

type LLMNodeParams struct {
	NodeID       string  `json:"node_id"`
	Model        string  `json:"model"`
	MaxTokens    int     `json:"max_tokens"`
	Temperature  float32 `json:"temperature"`
	SystemPrompt string  `json:"system_prompt"`
}

func (e *AIAgentExecutorV2) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return e.actionManager.Run(ctx, params.ActionType, params)
}

func (e *AIAgentExecutorV2) ProcessFunctionCalling(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	executeParams := ExecuteParams{}

	err := e.parameterBinder.BindToStruct(ctx, item, &executeParams, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	action := domain.IntegrationAction{}

	for _, schemaAction := range schema.Actions {
		if schemaAction.ActionType == IntegrationActionType_FunctionCallingAgent {
			action = schemaAction
			break
		}
	}

	handles := action.HandlesByContext[domain.UsageContextWorkflow]

	if len(handles.Input) < 4 {
		return nil, fmt.Errorf("agent node %s has less than 4 input handles", params.NodeID)
	}

	handleIdFormat := "input-%s-%d"

	llmHandleID := fmt.Sprintf(handleIdFormat, params.NodeID, 1)
	memoryHandleID := fmt.Sprintf(handleIdFormat, params.NodeID, 2)
	toolsHandleID := fmt.Sprintf(handleIdFormat, params.NodeID, 3)

	agentNode, exists := params.Workflow.GetActionNodeByID(params.NodeID)
	if !exists {
		return nil, fmt.Errorf("agent node %s not found in workflow", params.NodeID)
	}

	memoryNodeID := ""

	memoryInput, exists := agentNode.GetInputByID(memoryHandleID)
	if exists && len(memoryInput.SubscribedEvents) > 0 {
		memoryNodeID = e.GetNodeIDFromOutputID(memoryInput.SubscribedEvents[0])

		if memoryNodeID != "" {
			executeParams.Memory = &NodeReference{NodeID: memoryNodeID}
		}
	}

	toolsInput, exists := agentNode.GetInputByID(toolsHandleID)
	if exists {
		toolNodeIDs := e.GetNodeIDsFromOutputIDs(toolsInput.SubscribedEvents)

		executeParams.Tools = make([]NodeReference, 0, len(toolNodeIDs))

		for _, toolNodeID := range toolNodeIDs {
			executeParams.Tools = append(executeParams.Tools, NodeReference{NodeID: toolNodeID})
		}
	}

	llmInput, exists := agentNode.GetInputByID(llmHandleID)
	if !exists {
		return nil, fmt.Errorf("LLM input %s not found in agent node %s", llmHandleID, params.NodeID)
	}

	if len(llmInput.SubscribedEvents) == 0 {
		return nil, fmt.Errorf("LLM node is required")
	}

	llmNodeID := e.GetNodeIDFromOutputID(llmInput.SubscribedEvents[0])

	if llmNodeID == "" {
		return nil, fmt.Errorf("LLM node is required")
	}

	executeParams.LLM = &NodeReference{NodeID: llmNodeID}

	// Create item processor

	if executeParams.LLM == nil {
		return nil, fmt.Errorf("LLM configuration is required")
	}

	workflow := *params.Workflow

	agentSettings, err := e.ResolveAgentSettings(ctx, executeParams, workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent settings: %w", err)
	}

	// Build initial prompt with input context
	initialPrompt := executeParams.Prompt

	if initialPrompt == "" {
		return nil, fmt.Errorf("initial prompt is required")
	}

	workspaceID := params.Workflow.WorkspaceID

	llm := agentSettings.LLM
	memory := agentSettings.Memory
	tools := agentSettings.Tools

	stateManager := NewInMemoryFunctionCallingStateManager()

	toolCallManager := NewDefaultToolCallManager(DefaultToolCallManagerDeps{
		AgentNodeID:                params.NodeID,
		ExecutorIntegrationManager: e.executorIntegrationManager,
		ParameterBinder:            e.parameterBinder,
		EventPublisher:             e.eventPublisher,
	})

	memoryNodeParams := MemoryNodeParams{}

	if memory.Memory != nil {
		err := e.parameterBinder.BindToStruct(ctx, item, &memoryNodeParams, memory.Node.IntegrationSettings)
		if err != nil {
			return nil, fmt.Errorf("failed to bind memory node params: %w", err)
		}
	}

	llmNodeParams := LLMNodeParams{}

	if llm.LLM != nil {
		llmNodeParams.NodeID = llm.Node.ID
		err := e.parameterBinder.BindToStruct(ctx, item, &llmNodeParams, llm.Node.IntegrationSettings)
		if err != nil {
			return nil, fmt.Errorf("failed to bind LLM node params: %w", err)
		}
	}

	log.Debug().Interface("memory_node_params", memoryNodeParams).Msg("Memory node params")

	// Initialize memory manager
	memoryConfig := ConversationMemoryConfig{
		Enabled:           memory.Memory != nil,
		SessionID:         memoryNodeParams.SessionID,
		ConversationCount: memoryNodeParams.ConversationCount,
		IncludeToolUsage:  true,
		MaxContextLength:  memoryNodeParams.MaxContextLength,
	}

	if memory.Memory != nil {
		memoryNodeID = executeParams.Memory.NodeID
	}

	memoryManager := NewConversationMemoryManager(ConversationMemoryManagerDependencies{
		Memory:         memory.Memory,
		AgentNodeID:    params.NodeID,
		Config:         memoryConfig,
		EventPublisher: e.eventPublisher,
		MemoryNodeID:   memoryNodeID,
	})

	deps := FunctionCallingConversationManagerDeps{
		AgentNodeID:     params.NodeID,
		LLM:             llm.LLM,
		Memory:          memory.Memory,
		ToolExecutors:   tools,
		ToolCallManager: toolCallManager,
		StateManager:    stateManager,
		EventPublisher:  e.eventPublisher,
		ExecuteParams:   executeParams,
		MemoryManager:   memoryManager,
		LLMNodeParams:   llmNodeParams,
		MemoryNodeID:    memoryNodeID,
	}

	fcManager := NewFunctionCallingConversationManager(deps)

	conversationID := fmt.Sprintf("fc_session_%d", time.Now().UnixNano())

	_, err = fcManager.ExecuteFunctionCallingConversation(ctx, conversationID, workspaceID, initialPrompt)
	if err != nil {
		return nil, fmt.Errorf("function calling conversation failed: %w", err)
	}

	err = e.publishConnectedNodeEventsAfterExecution(ctx, executeParams, workflow.ID, params.NodeID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to publish connected node events")
	}

	if execCtx, ok := domain.GetWorkflowExecutionContext(ctx); ok && execCtx.AgentTracker != nil {
		executionStats := execCtx.AgentTracker.GetExecutionStatistics()
	}

	return []domain.Item{}, nil
}

type AgentSettings struct {
	LLM    ResolveLLMResult
	Memory ResolveMemoryResult
	Tools  []ToolExecutor
}

func (e *AIAgentExecutorV2) ResolveAgentSettings(ctx context.Context, executeParams ExecuteParams, workflow domain.Workflow) (AgentSettings, error) {
	llm, err := e.ResolveLLM(ctx, executeParams.LLM, workflow)
	if err != nil {
		return AgentSettings{}, fmt.Errorf("failed to resolve LLM: %w", err)
	}

	memory, err := e.ResolveMemory(ctx, executeParams.Memory, workflow)
	if err != nil {
		return AgentSettings{}, fmt.Errorf("failed to resolve memory: %w", err)
	}

	tools, err := e.ResolveTools(ctx, executeParams.Tools, workflow)
	if err != nil {
		return AgentSettings{}, fmt.Errorf("failed to resolve tools: %w", err)
	}

	return AgentSettings{
		LLM:    llm,
		Memory: memory,
		Tools:  tools,
	}, nil
}

type ResolveLLMResult struct {
	LLM  domain.IntegrationLLM
	Node domain.WorkflowNode
}

func (e *AIAgentExecutorV2) ResolveLLM(ctx context.Context, llmRef *NodeReference, workflow domain.Workflow) (ResolveLLMResult, error) {
	if llmRef == nil {
		return ResolveLLMResult{}, fmt.Errorf("LLM reference is required")
	}

	llmNode, exists := workflow.GetActionNodeByID(llmRef.NodeID)
	if !exists {
		return ResolveLLMResult{}, fmt.Errorf("attached LLM node %s not found in workflow", llmRef.NodeID)
	}

	creator, err := e.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
		IntegrationType: domain.IntegrationType(llmNode.NodeType),
	})
	if err != nil {
		return ResolveLLMResult{}, fmt.Errorf("failed to resolve LLM: %w", err)
	}

	credentialID, exists := llmNode.IntegrationSettings["credential_id"]
	if !exists {
		return ResolveLLMResult{}, fmt.Errorf("credential_id is not found in LLM node %s", llmRef.NodeID)
	}

	credentialIDString, ok := credentialID.(string)
	if !ok {
		return ResolveLLMResult{}, fmt.Errorf("credential_id is not a string in LLM node %s", llmRef.NodeID)
	}

	executor, err := creator.CreateIntegration(ctx, domain.CreateIntegrationParams{
		WorkspaceID:  workflow.WorkspaceID,
		CredentialID: credentialIDString,
	})
	if err != nil {
		return ResolveLLMResult{}, fmt.Errorf("failed to create LLM: %w", err)
	}

	llm, ok := executor.(domain.IntegrationLLM)
	if !ok {
		return ResolveLLMResult{}, fmt.Errorf("LLM is not a domain.IntegrationLLM")
	}

	return ResolveLLMResult{
		LLM:  llm,
		Node: llmNode,
	}, nil
}

type ResolveMemoryResult struct {
	Memory domain.IntegrationMemory
	Node   domain.WorkflowNode
}

func (e *AIAgentExecutorV2) ResolveMemory(ctx context.Context, memoryRef *NodeReference, workflow domain.Workflow) (ResolveMemoryResult, error) {
	if memoryRef == nil {
		return ResolveMemoryResult{}, nil
	}

	memoryNode, exists := workflow.GetActionNodeByID(memoryRef.NodeID)
	if !exists {
		return ResolveMemoryResult{}, fmt.Errorf("attached memory node %s not found in workflow", memoryRef.NodeID)
	}

	creator, err := e.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
		IntegrationType: domain.IntegrationType(memoryNode.NodeType),
	})
	if err != nil {
		return ResolveMemoryResult{}, fmt.Errorf("failed to resolve memory: %w", err)
	}

	var credentialID string

	credentialIDValue, exists := memoryNode.IntegrationSettings["credential_id"]
	if exists {
		credentialIDString, ok := credentialIDValue.(string)
		if !ok {
			return ResolveMemoryResult{}, fmt.Errorf("credential_id is not a string in memory node %s", memoryRef.NodeID)
		}

		credentialID = credentialIDString
	}

	memory, err := creator.CreateIntegration(ctx, domain.CreateIntegrationParams{
		WorkspaceID:  workflow.WorkspaceID,
		CredentialID: credentialID,
	})
	if err != nil {
		return ResolveMemoryResult{}, fmt.Errorf("failed to create memory: %w", err)
	}

	memoryExecutor, ok := memory.(domain.IntegrationMemory)
	if !ok {
		return ResolveMemoryResult{}, fmt.Errorf("memory is not a domain.IntegrationMemory")
	}

	return ResolveMemoryResult{
		Memory: memoryExecutor,
		Node:   memoryNode,
	}, nil
}

func (e *AIAgentExecutorV2) ResolveTools(ctx context.Context, toolRefs []NodeReference, workflow domain.Workflow) ([]ToolExecutor, error) {
	var tools []ToolExecutor

	for _, toolRef := range toolRefs {
		toolNode, exists := workflow.GetActionNodeByID(toolRef.NodeID)
		if !exists {
			return nil, fmt.Errorf("attached tool node %s not found in workflow", toolRef.NodeID)
		}

		log.Debug().
			Interface("tool_node", toolNode).
			Msg("Resolving tool")

		creator, err := e.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
			IntegrationType: domain.IntegrationType(toolNode.NodeType),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve tool %s: %w", toolNode.Name, err)
		}

		credentialID, exists := toolNode.IntegrationSettings["credential_id"]
		if !exists {
			log.Error().Msgf("credential_id is not found in tool node %s", toolRef.NodeID)
			credentialID = ""
		}

		credentialIDString, ok := credentialID.(string)
		if !ok {
			log.Error().Msgf("credential_id is not a string in tool node %s", toolRef.NodeID)
			continue
		}

		executor, err := creator.CreateIntegration(ctx, domain.CreateIntegrationParams{
			WorkspaceID:  workflow.WorkspaceID,
			CredentialID: credentialIDString,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create tool %s: %w", toolNode.Name, err)
		}

		// Create ToolExecutor with metadata, restricting to only this node's action
		toolExecutor := ToolExecutor{
			Executor:        executor,
			IntegrationType: domain.IntegrationType(toolNode.NodeType),
			NodeID:          toolRef.NodeID,
			NodeName:        toolNode.Name,
			CredentialID:    credentialIDString,
			WorkspaceID:     workflow.WorkspaceID,
			// Only allow the specific action type for this workflow node
			AllowedActions: []domain.IntegrationActionType{toolNode.ActionType},
			// Include the full workflow node for parameter resolution
			WorkflowNode: &toolNode,
		}

		tools = append(tools, toolExecutor)
	}

	return tools, nil
}

func (e *AIAgentExecutorV2) GetNodeIDsFromOutputIDs(outputIDs []string) []string {
	nodeIDs := make([]string, 0, len(outputIDs))

	for _, outputID := range outputIDs {
		parts := strings.Split(outputID, "-")

		if len(parts) >= 3 {
			nodeID := e.GetNodeIDFromOutputID(outputID)

			nodeIDs = append(nodeIDs, nodeID)
		}
	}

	return nodeIDs
}

func (e *AIAgentExecutorV2) GetNodeIDFromOutputID(outputID string) string {
	parts := strings.Split(outputID, "-")

	if len(parts) >= 3 {
		return strings.Join(parts[1:len(parts)-1], "-")
	}

	return ""
}

func (e *AIAgentExecutorV2) publishConnectedNodeEventsAfterExecution(ctx context.Context, executeParams ExecuteParams, workflowID string, agentNodeID string) error {
	executionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return fmt.Errorf("workflow execution context not found")
	}

	executedToolNodeIDs := make(map[string]bool)
	if executionContext != nil && executionContext.AgentTracker != nil {
		for _, nodeID := range executionContext.AgentTracker.GetExecutedNodeIDsByType(domain.NodeTypeTool) {
			executedToolNodeIDs[nodeID] = true
		}
	}

	if executeParams.LLM != nil {
		e.publishNodeEvents(ctx, executeParams.LLM.NodeID, workflowID, executionContext.WorkflowExecutionID, agentNodeID)
	}

	if executeParams.Memory != nil {
		e.publishNodeEvents(ctx, executeParams.Memory.NodeID, workflowID, executionContext.WorkflowExecutionID, agentNodeID)
	}

	for nodeID := range executedToolNodeIDs {
		e.publishNodeEvents(ctx, nodeID, workflowID, executionContext.WorkflowExecutionID, agentNodeID)
	}

	return nil
}

func (e *AIAgentExecutorV2) publishNodeEvents(ctx context.Context, nodeID, workflowID, executionID string, agentNodeID string) {
	executionContext, hasContext := domain.GetWorkflowExecutionContext(ctx)

	var itemsByInputID map[string]domain.NodeItems
	var itemsByOutputID map[string]domain.NodeItems

	if hasContext && executionContext.AgentTracker != nil {
		if aggregated := executionContext.AgentTracker.GetAggregatedExecution(nodeID); aggregated != nil {
			itemsByInputID = make(map[string]domain.NodeItems)
			if len(aggregated.InputItems) > 0 {
				inputID := fmt.Sprintf("input-%s-0", nodeID)
				itemsByInputID[inputID] = domain.NodeItems{
					FromNodeID: agentNodeID,
					Items:      aggregated.InputItems,
				}
			}

			itemsByOutputID = make(map[string]domain.NodeItems)
			if len(aggregated.OutputItems) > 0 {
				outputID := fmt.Sprintf("output-%s-0", nodeID)
				itemsByOutputID[outputID] = domain.NodeItems{
					FromNodeID: nodeID,
					Items:      aggregated.OutputItems,
				}
			}
		}
	}

	if itemsByInputID == nil {
		itemsByInputID = map[string]domain.NodeItems{}
	}
	if itemsByOutputID == nil {
		itemsByOutputID = map[string]domain.NodeItems{}
	}

	if hasContext && executionContext.HistoryRecorder != nil {
		historyEntry := domain.NodeExecutionEntry{
			NodeID:          nodeID,
			ItemsByInputID:  itemsByInputID,
			ItemsByOutputID: itemsByOutputID,
			EventType:       domain.NodeExecuted,
			Timestamp:       time.Now().UnixNano(),
			ExecutionOrder:  0,
		executionContext.HistoryRecorder.AddNodeExecutionEntry(historyEntry)
	}

	startedEvent := &domain.NodeExecutionStartedEvent{
		WorkflowID:          workflowID,
		WorkflowExecutionID: executionID,
		NodeID:              nodeID,
		Timestamp:           time.Now().UnixNano(),
	}

	err := e.eventPublisher.PublishEvent(ctx, startedEvent)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to publish node execution started event for node %s", nodeID)
		return
	}

	completedEvent := &domain.NodeExecutedEvent{
		WorkflowID:          workflowID,
		WorkflowExecutionID: executionID,
		NodeID:              nodeID,
		Timestamp:           time.Now().UnixNano(),
		ItemsByInputID:      itemsByInputID,
		ItemsByOutputID:     itemsByOutputID,
		ExecutionOrder:      0, // Connected nodes don't have execution order
	}

	err = e.eventPublisher.PublishEvent(ctx, completedEvent)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to publish node execution completed event for node %s", nodeID)
	}
}

func (e *AIAgentExecutorV2) convertPayloadToItems(payloadByInputID map[string]domain.Payload, fromNodeID string) map[string]domain.NodeItems {
	itemsByInputID := make(map[string]domain.NodeItems)

	for inputID, payload := range payloadByInputID {
		items, err := payload.ToItems()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to convert payload to items for input %s", inputID)
			continue
		}

		itemsByInputID[inputID] = domain.NodeItems{
			FromNodeID: fromNodeID,
			Items:      items,
		}
	}

	return itemsByInputID
}

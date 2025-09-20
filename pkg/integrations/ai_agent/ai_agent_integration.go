package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/xid"
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
	llmExecutionCounts         map[string]int
	memoryExecutionCounts      map[string]int
}

func NewAIAgentExecutorV2(deps domain.IntegrationDeps) domain.IntegrationExecutor {
	executor := &AIAgentExecutorV2{
		integrationSelector:        deps.IntegrationSelector,
		parameterBinder:            deps.ParameterBinder,
		executorIntegrationManager: deps.ExecutorIntegrationManager,
		eventPublisher:             deps.ExecutorEventPublisher,
		llmExecutionCounts:         make(map[string]int),
		memoryExecutionCounts:      make(map[string]int),
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

func (e *AIAgentExecutorV2) incrementLLMExecutionCount(nodeID string) {
	if existing, exists := e.llmExecutionCounts[nodeID]; exists {
		e.llmExecutionCounts[nodeID] = existing + 1
	} else {
		e.llmExecutionCounts[nodeID] = 1
	}
}

func (e *AIAgentExecutorV2) incrementMemoryExecutionCount(nodeID string) {
	if existing, exists := e.memoryExecutionCounts[nodeID]; exists {
		e.memoryExecutionCounts[nodeID] = existing + 1
	} else {
		e.memoryExecutionCounts[nodeID] = 1
	}
}

func (e *AIAgentExecutorV2) ProcessFunctionCalling(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	e.llmExecutionCounts = make(map[string]int)
	e.memoryExecutionCounts = make(map[string]int)

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
	itemProcessor := NewItemProcessor(e.parameterBinder)

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

	// Process input items from upstream nodes
	inputItems, err := itemProcessor.ProcessInputItems(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to process input items: %w", err)
	}

	// Add input context to prompt if available, FIXME: Enes: Do we need this really?
	inputContext := itemProcessor.ExtractPromptContext(inputItems)
	if inputContext != "" {
		initialPrompt = fmt.Sprintf("%s\n\n%s", initialPrompt, inputContext)
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
		AgentNodeID:             params.NodeID,
		LLM:                     llm.LLM,
		Memory:                  memory.Memory,
		ToolExecutors:           tools,
		ToolCallManager:         toolCallManager,
		StateManager:            stateManager,
		EventPublisher:          e.eventPublisher,
		ExecuteParams:           executeParams,
		MemoryManager:           memoryManager,
		LLMNodeParams:           llmNodeParams,
		MemoryNodeID:            memoryNodeID,
		LLMExecutionCallback:    e.incrementLLMExecutionCount,
		MemoryExecutionCallback: e.incrementMemoryExecutionCount,
	}

	fcManager := NewFunctionCallingConversationManager(deps)

	conversationID := fmt.Sprintf("fc_session_%d", time.Now().UnixNano())

	result, err := fcManager.ExecuteFunctionCallingConversation(ctx, conversationID, workspaceID, initialPrompt)
	if err != nil {
		return nil, fmt.Errorf("function calling conversation failed: %w", err)
	}

	// Publish events and record history for connected nodes
	publishParams := PublishEventsParams{
		ExecuteParams:    executeParams,
		AgentSettings:    agentSettings,
		ToolExecutions:   result.ToolExecutions,
		WorkspaceID:      workflow.WorkspaceID,
		WorkflowID:       workflow.ID,
		PayloadByInputID: params.PayloadByInputID,
		AgentNodeID:      params.NodeID,
	}
	err = e.publishConnectedNodeEventsAfterExecution(ctx, publishParams)
	if err != nil {
		log.Error().Err(err).Msg("Failed to publish connected node events")
	}

	// Create output items from conversation result
	outputItems, err := itemProcessor.CreateOutputItems(ctx, result, result.ToolExecutions)
	if err != nil {
		return nil, fmt.Errorf("failed to create output items: %w", err)
	}

	return outputItems, nil
}

type AgentSettings struct {
	LLM    ResolveLLMResult
	Memory ResolveMemoryResult
	Tools  []ToolExecutor
}

// PublishEventsParams encapsulates parameters for publishConnectedNodeEventsAfterExecution
type PublishEventsParams struct {
	ExecuteParams    ExecuteParams
	AgentSettings    AgentSettings
	ToolExecutions   []interface{}
	WorkspaceID      string
	WorkflowID       string
	PayloadByInputID map[string]domain.Payload
	AgentNodeID      string
}

// RecordHistoryParams encapsulates parameters for recordExecutionHistory
type RecordHistoryParams struct {
	ExecuteParams       ExecuteParams
	AgentSettings       AgentSettings
	ExecutedToolNodeIDs map[string]bool
	Recorder            domain.ExecutionHistoryRecorder
	PayloadByInputID    map[string]domain.Payload
	AgentNodeID         string
}

// NodeExecutionHistoryParams encapsulates parameters for recordNodeExecutionHistory
type NodeExecutionHistoryParams struct {
	NodeID          string
	IntegrationType string
	ActionType      string
	Recorder        domain.ExecutionHistoryRecorder
	ItemsByInputID  map[string]domain.NodeItems
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

func (e *AIAgentExecutorV2) publishConnectedNodeEventsAfterExecution(ctx context.Context, params PublishEventsParams) error {
	executionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return fmt.Errorf("workflow execution context not found")
	}

	executedToolNodeIDs := e.getExecutedToolNodeIDs(executionContext, params)

	// Publish events for LLM (always used)
	if params.ExecuteParams.LLM != nil {
		e.publishNodeEvents(ctx, params.ExecuteParams.LLM.NodeID, params.WorkflowID, executionContext.WorkflowExecutionID)
	}

	// Publish events for Memory if present
	if params.ExecuteParams.Memory != nil {
		e.publishNodeEvents(ctx, params.ExecuteParams.Memory.NodeID, params.WorkflowID, executionContext.WorkflowExecutionID)
	}

	// Publish events for executed tools only
	for nodeID := range executedToolNodeIDs {
		e.publishNodeEvents(ctx, nodeID, params.WorkflowID, executionContext.WorkflowExecutionID)
	}

	// Record execution history
	if executionContext.HistoryRecorder != nil {
		historyParams := RecordHistoryParams{
			ExecuteParams:       params.ExecuteParams,
			AgentSettings:       params.AgentSettings,
			ExecutedToolNodeIDs: executedToolNodeIDs,
			Recorder:            executionContext.HistoryRecorder,
			PayloadByInputID:    params.PayloadByInputID,
			AgentNodeID:         params.AgentNodeID,
		}
		e.recordExecutionHistory(ctx, historyParams)
	}

	return nil
}

func (e *AIAgentExecutorV2) publishNodeEvents(ctx context.Context, nodeID, workflowID, executionID string) {
	executionContext, hasContext := domain.GetWorkflowExecutionContext(ctx)

	var itemsByInputID map[string]domain.NodeItems
	var itemsByOutputID map[string]domain.NodeItems

	if hasContext && executionContext.ToolTracker != nil {
		executionContext.ToolTracker.ForEachExecution(func(trackedNodeID string, toolExec *domain.ToolExecution) {
			if trackedNodeID == nodeID {
				itemsByInputID = make(map[string]domain.NodeItems)
				if len(toolExec.InputItems) > 0 {
					inputID := fmt.Sprintf("input-%s-0", nodeID)
					itemsByInputID[inputID] = domain.NodeItems{
						FromNodeID: nodeID,
						Items:      toolExec.InputItems,
					}
				}

				itemsByOutputID = make(map[string]domain.NodeItems)
				if len(toolExec.OutputItems) > 0 {
					outputID := fmt.Sprintf("output-%s-0", nodeID)
					itemsByOutputID[outputID] = domain.NodeItems{
						FromNodeID: nodeID,
						Items:      toolExec.OutputItems,
					}
				}
			}
		})
	}

	if itemsByInputID == nil {
		itemsByInputID = map[string]domain.NodeItems{}
	}
	if itemsByOutputID == nil {
		itemsByOutputID = map[string]domain.NodeItems{}
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

func (e *AIAgentExecutorV2) recordExecutionHistory(ctx context.Context, params RecordHistoryParams) {
	itemsByInputID := e.convertPayloadToItems(params.PayloadByInputID, params.AgentNodeID)

	e.recordLLMExecutionIfPresent(params.ExecuteParams.LLM, params.Recorder, itemsByInputID)
	e.recordMemoryExecutionIfPresent(params.ExecuteParams.Memory, params.Recorder, itemsByInputID)
	e.recordToolExecutions(ctx, params.AgentSettings, params.ExecutedToolNodeIDs, params.Recorder, itemsByInputID, params.AgentNodeID)
}

func (e *AIAgentExecutorV2) recordLLMExecutionIfPresent(llm *NodeReference, recorder domain.ExecutionHistoryRecorder, itemsByInputID map[string]domain.NodeItems) {
	if llm == nil {
		return
	}

	count := e.llmExecutionCounts[llm.NodeID]
	if count == 0 {
		count = 1
	}

	llmItemsByInputID := make(map[string]domain.NodeItems)
	inputID := fmt.Sprintf("input-%s-0", llm.NodeID)

	var agentNodeID string
	for _, items := range itemsByInputID {
		agentNodeID = items.FromNodeID
		break
	}

	inputItems := make([]domain.Item, count)
	for i := 0; i < count; i++ {
		inputItems[i] = domain.Item(map[string]interface{}{"prompt": "LLM processing request"})
	}

	llmItemsByInputID[inputID] = domain.NodeItems{
		FromNodeID: agentNodeID,
		Items:      inputItems,
	}

	llmItemsByOutputID := make(map[string]domain.NodeItems)
	outputID := fmt.Sprintf("output-%s-0", llm.NodeID)

	outputItems := make([]domain.Item, count)
	for i := 0; i < count; i++ {
		outputItems[i] = domain.Item(map[string]interface{}{"response": "LLM response"})
	}

	llmItemsByOutputID[outputID] = domain.NodeItems{
		FromNodeID: llm.NodeID,
		Items:      outputItems,
	}

	e.recordNodeExecutionHistoryWithOutput(NodeExecutionHistoryParams{
		NodeID:          llm.NodeID,
		IntegrationType: "openai",
		ActionType:      "chat_completion",
		Recorder:        recorder,
		ItemsByInputID:  llmItemsByInputID,
	}, llmItemsByOutputID)
}

func (e *AIAgentExecutorV2) recordMemoryExecutionIfPresent(memory *NodeReference, recorder domain.ExecutionHistoryRecorder, itemsByInputID map[string]domain.NodeItems) {
	if memory == nil {
		return
	}

	count := e.memoryExecutionCounts[memory.NodeID]
	if count == 0 {
		count = 1
	}

	memoryItemsByInputID := make(map[string]domain.NodeItems)
	inputID := fmt.Sprintf("input-%s-0", memory.NodeID)

	var agentNodeID string
	for _, items := range itemsByInputID {
		agentNodeID = items.FromNodeID
		break
	}

	inputItems := make([]domain.Item, count)
	for i := 0; i < count; i++ {
		inputItems[i] = domain.Item(map[string]interface{}{"conversation": "Memory storing conversation"})
	}

	memoryItemsByInputID[inputID] = domain.NodeItems{
		FromNodeID: agentNodeID,
		Items:      inputItems,
	}

	memoryItemsByOutputID := make(map[string]domain.NodeItems)
	outputID := fmt.Sprintf("output-%s-0", memory.NodeID)

	outputItems := make([]domain.Item, count)
	for i := 0; i < count; i++ {
		outputItems[i] = domain.Item(map[string]interface{}{"stored": "Memory stored successfully"})
	}

	memoryItemsByOutputID[outputID] = domain.NodeItems{
		FromNodeID: memory.NodeID,
		Items:      outputItems,
	}

	e.recordNodeExecutionHistoryWithOutput(NodeExecutionHistoryParams{
		NodeID:          memory.NodeID,
		IntegrationType: "flowbaker_agent_memory",
		ActionType:      "store_conversation",
		Recorder:        recorder,
		ItemsByInputID:  memoryItemsByInputID,
	}, memoryItemsByOutputID)
}

func (e *AIAgentExecutorV2) recordToolExecutions(ctx context.Context, agentSettings AgentSettings, executedToolNodeIDs map[string]bool, recorder domain.ExecutionHistoryRecorder, itemsByInputID map[string]domain.NodeItems, agentNodeID string) {
	execContext, hasContext := domain.GetWorkflowExecutionContext(ctx)

	if e.canUseDetailedToolTracker(hasContext, execContext) {
		e.recordDetailedToolExecutions(execContext, recorder, itemsByInputID, agentNodeID)
	} else {
		e.recordBasicToolExecutions(executedToolNodeIDs, agentSettings.Tools, recorder, itemsByInputID)
	}
}

func (e *AIAgentExecutorV2) canUseDetailedToolTracker(hasContext bool, execContext *domain.WorkflowExecutionContext) bool {
	return hasContext && execContext != nil && execContext.ToolTracker != nil
}

func (e *AIAgentExecutorV2) recordDetailedToolExecutions(execContext *domain.WorkflowExecutionContext, recorder domain.ExecutionHistoryRecorder, itemsByInputID map[string]domain.NodeItems, agentNodeID string) {
	execContext.ToolTracker.ForEachExecution(func(nodeID string, toolExec *domain.ToolExecution) {
		toolItemsByInputID := make(map[string]domain.NodeItems)
		inputID := fmt.Sprintf("input-%s-0", nodeID)
		if len(toolExec.InputItems) > 0 {
			toolItemsByInputID[inputID] = domain.NodeItems{
				FromNodeID: agentNodeID,
				Items:      toolExec.InputItems,
			}
		}

		toolItemsByOutputID := make(map[string]domain.NodeItems)
		outputID := fmt.Sprintf("output-%s-0", nodeID)
		if len(toolExec.OutputItems) > 0 {
			toolItemsByOutputID[outputID] = domain.NodeItems{
				FromNodeID: nodeID,
				Items:      toolExec.OutputItems,
			}
		}

		e.recordNodeExecutionHistoryWithOutput(NodeExecutionHistoryParams{
			NodeID:          nodeID,
			IntegrationType: string(toolExec.Identifier.IntegrationType),
			ActionType:      string(toolExec.Identifier.ActionType),
			Recorder:        recorder,
			ItemsByInputID:  toolItemsByInputID,
		}, toolItemsByOutputID)
	})
}

func (e *AIAgentExecutorV2) recordBasicToolExecutions(executedToolNodeIDs map[string]bool, tools []ToolExecutor, recorder domain.ExecutionHistoryRecorder, itemsByInputID map[string]domain.NodeItems) {
	for nodeID := range executedToolNodeIDs {
		if tool := e.findToolByNodeID(nodeID, tools); tool != nil {
			e.recordNodeExecutionHistory(NodeExecutionHistoryParams{
				NodeID:          nodeID,
				IntegrationType: string(tool.IntegrationType),
				ActionType:      string(tool.WorkflowNode.ActionType),
				Recorder:        recorder,
				ItemsByInputID:  itemsByInputID,
			})
		}
	}
}

func (e *AIAgentExecutorV2) findToolByNodeID(nodeID string, tools []ToolExecutor) *ToolExecutor {
	for _, tool := range tools {
		if tool.NodeID == nodeID {
			return &tool
		}
	}
	return nil
}

func (e *AIAgentExecutorV2) getExecutedToolNodeIDs(executionContext *domain.WorkflowExecutionContext, params PublishEventsParams) map[string]bool {
	executedToolNodeIDs := e.getToolNodeIDsFromTracker(executionContext)

	if e.shouldUseLegacyResolution(executedToolNodeIDs) {
		executedToolNodeIDs = e.getToolNodeIDsFromExecutions(params)
	}

	return executedToolNodeIDs
}

func (e *AIAgentExecutorV2) getToolNodeIDsFromTracker(executionContext *domain.WorkflowExecutionContext) map[string]bool {
	executedToolNodeIDs := make(map[string]bool)

	if !e.hasToolTracker(executionContext) {
		return executedToolNodeIDs
	}

	for _, nodeID := range executionContext.ToolTracker.GetExecutedNodeIDs() {
		executedToolNodeIDs[nodeID] = true
	}

	return executedToolNodeIDs
}

func (e *AIAgentExecutorV2) hasToolTracker(executionContext *domain.WorkflowExecutionContext) bool {
	return executionContext != nil && executionContext.ToolTracker != nil
}

func (e *AIAgentExecutorV2) shouldUseLegacyResolution(executedToolNodeIDs map[string]bool) bool {
	return len(executedToolNodeIDs) == 0
}

func (e *AIAgentExecutorV2) getToolNodeIDsFromExecutions(params PublishEventsParams) map[string]bool {
	toolLookup := e.buildToolLookupMap(params.AgentSettings.Tools)
	executedToolNodeIDs := make(map[string]bool)

	for _, toolExecInterface := range params.ToolExecutions {
		if nodeID := e.extractNodeIDFromExecution(toolExecInterface, toolLookup); nodeID != "" {
			executedToolNodeIDs[nodeID] = true
		}
	}

	return executedToolNodeIDs
}

func (e *AIAgentExecutorV2) buildToolLookupMap(tools []ToolExecutor) map[string]string {
	toolLookup := make(map[string]string)

	for _, tool := range tools {
		toolName := e.generateToolName(tool)
		toolLookup[toolName] = tool.NodeID
	}

	return toolLookup
}

func (e *AIAgentExecutorV2) generateToolName(tool ToolExecutor) string {
	return fmt.Sprintf("%s_%s", tool.IntegrationType, tool.WorkflowNode.ActionType)
}

func (e *AIAgentExecutorV2) extractNodeIDFromExecution(toolExecInterface interface{}, toolLookup map[string]string) string {
	toolExec, ok := toolExecInterface.(FunctionCallExecution)
	if !ok {
		return ""
	}

	if e.hasMetadataNodeID(toolExec) {
		return toolExec.Metadata.NodeID
	}

	if nodeID, exists := toolLookup[toolExec.ToolName]; exists {
		return nodeID
	}

	return ""
}

func (e *AIAgentExecutorV2) hasMetadataNodeID(toolExec FunctionCallExecution) bool {
	return toolExec.Metadata != nil && toolExec.Metadata.NodeID != ""
}

func (e *AIAgentExecutorV2) recordNodeExecutionHistory(params NodeExecutionHistoryParams) {
	e.recordNodeExecutionHistoryWithOutput(params, map[string]domain.NodeItems{})
}

func (e *AIAgentExecutorV2) recordNodeExecutionHistoryWithOutput(params NodeExecutionHistoryParams, itemsByOutputID map[string]domain.NodeItems) {
	now := time.Now()

	inputItemsCount, inputItemsSizeInBytes := e.calculateInputMetrics(params.ItemsByInputID)
	outputItemsCount, outputItemsSizeInBytes := e.calculateOutputMetrics(itemsByOutputID)

	e.recordNodeExecution(params, now, inputItemsCount, inputItemsSizeInBytes, outputItemsCount, outputItemsSizeInBytes)
	e.recordNodeExecutionEntry(params, itemsByOutputID, now)
}

func (e *AIAgentExecutorV2) calculateInputMetrics(itemsByInputID map[string]domain.NodeItems) (domain.InputItemsCount, domain.InputItemsSizeInBytes) {
	inputItemsCount := make(domain.InputItemsCount)
	inputItemsSizeInBytes := make(domain.InputItemsSizeInBytes)

	for inputID, nodeItems := range itemsByInputID {
		inputItemsCount[inputID] = int64(len(nodeItems.Items))
		inputItemsSizeInBytes[inputID] = e.calculateItemsSize(nodeItems.Items)
	}

	return inputItemsCount, inputItemsSizeInBytes
}

func (e *AIAgentExecutorV2) calculateOutputMetrics(itemsByOutputID map[string]domain.NodeItems) (domain.OutputItemsCount, domain.OutputItemsSizeInBytes) {
	outputItemsCount := make(domain.OutputItemsCount)
	outputItemsSizeInBytes := make(domain.OutputItemsSizeInBytes)

	for outputID, nodeItems := range itemsByOutputID {
		if index := e.extractIndexFromOutputID(outputID); index >= 0 {
			outputItemsCount[index] = int64(len(nodeItems.Items))
			outputItemsSizeInBytes[index] = e.calculateItemsSize(nodeItems.Items)
		}
	}

	return outputItemsCount, outputItemsSizeInBytes
}

func (e *AIAgentExecutorV2) calculateItemsSize(items []domain.Item) int64 {
	var totalSize int64
	for _, item := range items {
		if itemBytes, err := json.Marshal(item); err == nil {
			totalSize += int64(len(itemBytes))
		}
	}
	return totalSize
}

func (e *AIAgentExecutorV2) extractIndexFromOutputID(outputID string) int64 {
	parts := strings.Split(outputID, "-")
	if len(parts) >= 3 {
		if index, err := strconv.ParseInt(parts[len(parts)-1], 10, 64); err == nil {
			return index
		}
	}
	return -1
}

func (e *AIAgentExecutorV2) recordNodeExecution(params NodeExecutionHistoryParams, now time.Time, inputItemsCount domain.InputItemsCount, inputItemsSizeInBytes domain.InputItemsSizeInBytes, outputItemsCount domain.OutputItemsCount, outputItemsSizeInBytes domain.OutputItemsSizeInBytes) {
	params.Recorder.AddNodeExecution(domain.NodeExecution{
		ID:                     xid.New().String(),
		NodeID:                 params.NodeID,
		IntegrationType:        domain.IntegrationType(params.IntegrationType),
		IntegrationActionType:  domain.IntegrationActionType(params.ActionType),
		StartedAt:              now,
		EndedAt:                now,
		ExecutionOrder:         0,
		InputItemsCount:        inputItemsCount,
		InputItemsSizeInBytes:  inputItemsSizeInBytes,
		OutputItemsCount:       outputItemsCount,
		OutputItemsSizeInBytes: outputItemsSizeInBytes,
	})
}

func (e *AIAgentExecutorV2) recordNodeExecutionEntry(params NodeExecutionHistoryParams, itemsByOutputID map[string]domain.NodeItems, now time.Time) {
	params.Recorder.AddNodeExecutionEntry(domain.NodeExecutionEntry{
		NodeID:          params.NodeID,
		ItemsByInputID:  params.ItemsByInputID,
		ItemsByOutputID: itemsByOutputID,
		EventType:       domain.NodeExecuted,
		Timestamp:       now.UnixNano(),
		ExecutionOrder:  0,
	})
}

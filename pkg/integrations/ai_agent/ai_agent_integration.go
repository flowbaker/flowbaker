package ai_agent

import (
	"context"
	"flowbaker/internal/domain"
	"fmt"
	"time"

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

	memoryNodeID := ""
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
	}

	fcManager := NewFunctionCallingConversationManager(deps)

	conversationID := fmt.Sprintf("fc_session_%d", time.Now().UnixNano())

	result, err := fcManager.ExecuteFunctionCallingConversation(ctx, conversationID, workspaceID, initialPrompt)
	if err != nil {
		return nil, fmt.Errorf("function calling conversation failed: %w", err)
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

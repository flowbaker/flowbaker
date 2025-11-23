package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/agent"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider/openai"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/tool"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/flowbaker/flowbaker/pkg/domain/executor"

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
	executorCredentialManager  domain.ExecutorCredentialManager
}

func NewAIAgentCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &AIAgentCreator{
		integrationSelector:        deps.IntegrationSelector,
		parameterBinder:            deps.ParameterBinder,
		executorIntegrationManager: deps.ExecutorIntegrationManager,
		executorCredentialManager:  deps.ExecutorCredentialManager,
	}
}

func (c *AIAgentCreator) CreateIntegration(ctx context.Context, params domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewAIAgentExecutorV2(domain.IntegrationDeps{
		IntegrationSelector:        c.integrationSelector,
		ParameterBinder:            c.parameterBinder,
		ExecutorIntegrationManager: c.executorIntegrationManager,
		ExecutorCredentialManager:  c.executorCredentialManager,
	}), nil
}

type OpenAICredential struct {
	APIKey string `json:"api_key"`
}

type AIAgentExecutorV2 struct {
	integrationSelector        domain.IntegrationSelector
	parameterBinder            domain.IntegrationParameterBinder
	executorIntegrationManager domain.ExecutorIntegrationManager
	actionManager              *domain.IntegrationActionManager
	credentialGetter           domain.CredentialGetter[OpenAICredential]
}

func NewAIAgentExecutorV2(deps domain.IntegrationDeps) domain.IntegrationExecutor {
	executor := &AIAgentExecutorV2{
		integrationSelector:        deps.IntegrationSelector,
		parameterBinder:            deps.ParameterBinder,
		executorIntegrationManager: deps.ExecutorIntegrationManager,
		credentialGetter:           managers.NewExecutorCredentialGetter[OpenAICredential](deps.ExecutorCredentialManager),
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

const HandleIDFormat = "input-%s-%d"

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

	llmHandleID := fmt.Sprintf(HandleIDFormat, params.NodeID, 1)
	memoryHandleID := fmt.Sprintf(HandleIDFormat, params.NodeID, 2)
	toolsHandleID := fmt.Sprintf(HandleIDFormat, params.NodeID, 3)

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
	/* 	itemProcessor := NewItemProcessor(e.parameterBinder)
	 */
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
	/* 	inputItems, err := itemProcessor.ProcessInputItems(ctx, params)
	   	if err != nil {
	   		return nil, fmt.Errorf("failed to process input items: %w", err)
	   	}
	*/
	// Add input context to prompt if available, FIXME: Enes: Do we need this really?
	/* 	inputContext := itemProcessor.ExtractPromptContext(inputItems)
	   	if inputContext != "" {
	   		initialPrompt = fmt.Sprintf("%s\n\n%s", initialPrompt, inputContext)
	   	} */

	/* 	workspaceID := params.Workflow.WorkspaceID
	 */
	llm := agentSettings.LLM
	memory := agentSettings.Memory

	executionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return nil, fmt.Errorf("workflow execution context not found")
	}

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

	toolCreator := NewIntegrationToolCreator(IntegrationToolCreatorDeps{
		IntegrationSelector:        e.integrationSelector,
		ExecutorIntegrationManager: e.executorIntegrationManager,
		ExecutionObserver:          executionContext.ExecutionObserver,
	})

	tools, err := toolCreator.CreateTools(ctx, executeParams.Tools, workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to create tools: %w", err)
	}

	a, err := agent.New(
		agent.WithModel(llm.LLM),
		agent.WithMaxTokens(4000),
		agent.WithTemperature(1),
		agent.WithSystemPrompt("You are a helpful assistant."),
		agent.WithMemory(memory.Memory),
		agent.WithTools(tools...),
		agent.WithMaxIterations(10),
		agent.WithCancelContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	result, err := a.ChatSync(ctx, agent.ChatRequest{
		Prompt:    initialPrompt,
		SessionID: "",
		UserID:    "",
	})
	if err != nil {
		log.Error().Str("err", err.Error()).Msg("failed to chat with agent")
		return nil, fmt.Errorf("failed to chat with agent: %w", err)
	}

	maxStepNumber := math.MinInt
	maxStep := &agent.Step{}

	for _, step := range result.Steps {
		log.Debug().Interface("step", step).Msg("Step")

		if step.StepNumber > maxStepNumber {
			maxStepNumber = step.StepNumber
			maxStep = step
		}
	}

	var outputItems []domain.Item

	if maxStep == nil {
		return nil, nil
	}

	if maxStep.Content == "" {
		return nil, nil
	}

	resultItem := map[string]any{
		"output": maxStep.Content,
	}

	outputItems = append(outputItems, resultItem)

	/* 	// Create output items from conversation result
	   	outputItems, err := itemProcessor.CreateOutputItems(ctx, result, result.ToolExecutions)
	   	if err != nil {
	   		return nil, fmt.Errorf("failed to create output items: %w", err)
	   	}
	*/

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
	LLM  provider.LanguageModel
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

	credentialID, exists := llmNode.IntegrationSettings["credential_id"]
	if !exists {
		return ResolveLLMResult{}, fmt.Errorf("credential_id is not found in LLM node %s", llmRef.NodeID)
	}

	credentialIDString, ok := credentialID.(string)
	if !ok {
		return ResolveLLMResult{}, fmt.Errorf("credential_id is not a string in LLM node %s", llmRef.NodeID)
	}

	model, exists := llmNode.IntegrationSettings["model"]
	if !exists {
		return ResolveLLMResult{}, fmt.Errorf("model is not found in LLM node %s", llmRef.NodeID)
	}

	modelString, ok := model.(string)
	if !ok {
		return ResolveLLMResult{}, fmt.Errorf("model is not a string in LLM node %s", llmRef.NodeID)
	}

	var languageModel provider.LanguageModel

	switch llmNode.NodeType {
	case "openai":
		credential, err := e.credentialGetter.GetDecryptedCredential(ctx, credentialIDString)
		if err != nil {
			return ResolveLLMResult{}, fmt.Errorf("failed to get credential: %w", err)
		}

		languageModel = openai.New(credential.APIKey, modelString)
	default:
		return ResolveLLMResult{}, fmt.Errorf("unsupported LLM node type: %s", llmNode.NodeType)
	}

	return ResolveLLMResult{
		LLM:  languageModel,
		Node: llmNode,
	}, nil
}

type ResolveMemoryResult struct {
	Memory memory.Store
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

	_, ok := memory.(domain.IntegrationMemory)
	if !ok {
		return ResolveMemoryResult{}, fmt.Errorf("memory is not a domain.IntegrationMemory")
	}

	return ResolveMemoryResult{
		/* 		Memory: memoryExecutor, */
		Node: memoryNode,
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

type IntegrationToolCreator struct {
	integrationSelector        domain.IntegrationSelector
	executorIntegrationManager domain.ExecutorIntegrationManager
	observer                   domain.ExecutionObserver
}

type IntegrationToolCreatorDeps struct {
	IntegrationSelector        domain.IntegrationSelector
	ExecutorIntegrationManager domain.ExecutorIntegrationManager
	ExecutionObserver          domain.ExecutionObserver
}

func NewIntegrationToolCreator(deps IntegrationToolCreatorDeps) *IntegrationToolCreator {
	return &IntegrationToolCreator{
		integrationSelector:        deps.IntegrationSelector,
		executorIntegrationManager: deps.ExecutorIntegrationManager,
		observer:                   deps.ExecutionObserver,
	}
}

func (c *IntegrationToolCreator) CreateTools(ctx context.Context, nodeReferences []NodeReference, workflow domain.Workflow) ([]tool.Tool, error) {
	tools := make([]tool.Tool, 0, len(nodeReferences))

	for _, nodeReference := range nodeReferences {
		toolNode, exists := workflow.GetActionNodeByID(nodeReference.NodeID)
		if !exists {
			return nil, fmt.Errorf("attached tool node %s not found in workflow", nodeReference.NodeID)
		}

		creator, err := c.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
			IntegrationType: domain.IntegrationType(toolNode.NodeType),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve tool %s: %w", toolNode.Name, err)
		}

		credentialID, exists := toolNode.IntegrationSettings["credential_id"]
		if !exists {
			return nil, fmt.Errorf("credential_id is not found in tool node %s", nodeReference.NodeID)
		}

		credentialIDString, ok := credentialID.(string)
		if !ok {
			return nil, fmt.Errorf("credential_id is not a string in tool node %s", nodeReference.NodeID)
		}

		integrationExecutor, err := creator.CreateIntegration(ctx, domain.CreateIntegrationParams{
			WorkspaceID:  workflow.WorkspaceID,
			CredentialID: credentialIDString,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create tool %s: %w", toolNode.Name, err)
		}

		actionTool, err := c.GetActionTool(ctx, toolNode)
		if err != nil {
			return nil, fmt.Errorf("failed to get action tool %s: %w", toolNode.Name, err)
		}

		log.Debug().Interface("action_tool", actionTool).Msg("Action tool")

		toolInputHandleID := fmt.Sprintf(HandleIDFormat, toolNode.ID, 0)

		executeFunc := func(args string) (string, error) {
			err = c.observer.Notify(ctx, executor.NodeExecutionStartedEvent{
				NodeID:    toolNode.ID,
				Timestamp: time.Now(),
			})
			if err != nil {
				return "", fmt.Errorf("ai agent integration failed to notify observer about tool execution started: %w", err)
			}

			var inputItem domain.Item

			log.Debug().Interface("args", args).Msg("Args")

			err := json.Unmarshal([]byte(args), &inputItem)
			if err != nil {
				return "", fmt.Errorf("failed to unmarshal tool input: %w", err)
			}

			log.Debug().Interface("input_item", inputItem).Msg("Input item")

			inputItems := []domain.Item{inputItem}

			inputPayload, err := json.Marshal(inputItems)
			if err != nil {
				return "", fmt.Errorf("failed to marshal tool input: %w", err)
			}

			log.Debug().Interface("input_payload", string(inputPayload)).Msg("Input payload")

			p := domain.IntegrationInput{
				NodeID:     toolNode.ID,
				ActionType: toolNode.ActionType,
				IntegrationParams: domain.IntegrationParams{
					Settings: toolNode.IntegrationSettings,
				},
				PayloadByInputID: map[string]domain.Payload{
					toolInputHandleID: domain.Payload(inputPayload),
				},
				Workflow: &workflow,
			}

			itemsByInputID := map[string]domain.NodeItems{
				toolInputHandleID: {
					FromNodeID: toolNode.ID,
					Items:      []domain.Item{inputItem},
				},
			}

			startTime := time.Now()

			output, err := integrationExecutor.Execute(ctx, p)
			if err != nil {
				log.Error().Err(err).Msg("Failed to execute tool")
				err = c.observer.Notify(ctx, executor.NodeExecutionFailedEvent{
					NodeID:         toolNode.ID,
					ItemsByInputID: itemsByInputID,
					Error:          err,
					Timestamp:      time.Now(),
				})
				if err != nil {
					return "", fmt.Errorf("ai agent integration failed to notify observer about tool execution failed: %w", err)
				}

				return "", fmt.Errorf("failed to execute tool %s: %w", toolNode.Name, err)
			}

			err = c.observer.Notify(ctx, executor.NodeExecutionCompletedEvent{
				NodeID:                toolNode.ID,
				ItemsByInputID:        itemsByInputID,
				ItemsByOutputID:       output.ToItemsByOutputID(toolNode.ID),
				StartedAt:             startTime,
				EndedAt:               time.Now(),
				IntegrationType:       toolNode.NodeType,
				IntegrationActionType: toolNode.ActionType,
			})
			if err != nil {
				return "", fmt.Errorf("ai agent integration failed to notify observer about tool execution completed: %w", err)
			}

			payloads := output.ResultJSONByOutputID

			lastPayload := payloads[len(payloads)-1]

			return string(lastPayload), nil
		}

		funcTool := tool.Define(actionTool.Name, actionTool.Description, actionTool.Parameters, executeFunc)

		tools = append(tools, funcTool)
	}

	return tools, nil
}

func (c *IntegrationToolCreator) GetActionTool(ctx context.Context, toolNode domain.WorkflowNode) (types.Tool, error) {
	integration, err := c.executorIntegrationManager.GetIntegration(ctx, domain.IntegrationType(toolNode.NodeType))
	if err != nil {
		return types.Tool{}, fmt.Errorf("failed to get integration for type %s: %w", toolNode.NodeType, err)
	}

	var action domain.IntegrationAction
	found := false
	for _, a := range integration.Actions {
		if a.ActionType == toolNode.ActionType {
			action = a
			found = true
			break
		}
	}
	if !found {
		return types.Tool{}, fmt.Errorf("action not found for type %s in integration %s", toolNode.ActionType, toolNode.NodeType)
	}

	properties := make(map[string]any)
	required := []string{}

	for _, prop := range action.Properties {
		propSchema := c.convertPropertyToJSONSchema(prop)
		properties[prop.Key] = propSchema

		if prop.Required {
			required = append(required, prop.Key)
		}
	}

	parameters := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		parameters["required"] = required
	}

	// Create the tool name in format: {integration_type}_{action_type}
	toolName := fmt.Sprintf("%s_%s", strings.ToLower(string(toolNode.NodeType)), strings.ToLower(string(action.ActionType)))

	return types.Tool{
		Name:        toolName,
		Description: fmt.Sprintf("%s: %s", action.Name, action.Description),
		Parameters:  parameters,
	}, nil
}

// convertPropertyToJSONSchema converts a domain.NodeProperty to a JSON Schema object
func (c *IntegrationToolCreator) convertPropertyToJSONSchema(prop domain.NodeProperty) map[string]any {
	schema := map[string]any{
		"type": c.mapPropertyTypeToJSONType(prop.Type),
	}

	// Add description
	if prop.Description != "" {
		schema["description"] = prop.Description
	}

	// Handle array types
	if prop.Type == domain.NodePropertyType_Array || prop.Type == domain.NodePropertyType_TagInput {
		if prop.ArrayOpts != nil {
			itemsSchema := map[string]any{
				"type": c.mapPropertyTypeToJSONType(prop.ArrayOpts.ItemType),
			}

			// Handle nested properties for array items (e.g., array of objects)
			if len(prop.ArrayOpts.ItemProperties) > 0 {
				itemsProperties := make(map[string]any)
				itemsRequired := []string{}

				for _, itemProp := range prop.ArrayOpts.ItemProperties {
					itemsProperties[itemProp.Key] = c.convertPropertyToJSONSchema(itemProp)
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

			// Add array constraints
			if prop.ArrayOpts.MinItems > 0 {
				schema["minItems"] = prop.ArrayOpts.MinItems
			}
			if prop.ArrayOpts.MaxItems > 0 {
				schema["maxItems"] = prop.ArrayOpts.MaxItems
			}
		}
	}

	// Handle enum options
	if len(prop.Options) > 0 {
		enumValues := make([]any, 0, len(prop.Options))
		for _, option := range prop.Options {
			enumValues = append(enumValues, option.Value)
		}
		schema["enum"] = enumValues
	}

	// Add string validation constraints
	if prop.MinLength > 0 {
		schema["minLength"] = prop.MinLength
	}
	if prop.MaxLength > 0 {
		schema["maxLength"] = prop.MaxLength
	}
	if prop.Pattern != "" {
		schema["pattern"] = prop.Pattern
	}

	return schema
}

// mapPropertyTypeToJSONType maps domain.NodePropertyType to JSON Schema type strings
func (c *IntegrationToolCreator) mapPropertyTypeToJSONType(propType domain.NodePropertyType) string {
	switch propType {
	case domain.NodePropertyType_String, domain.NodePropertyType_Text, domain.NodePropertyType_CodeEditor:
		return "string"
	case domain.NodePropertyType_Integer:
		return "integer"
	case domain.NodePropertyType_Number, domain.NodePropertyType_Float:
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

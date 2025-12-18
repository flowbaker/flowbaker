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
	return NewAIAgentExecutor(domain.IntegrationDeps{
		IntegrationSelector:        c.integrationSelector,
		ParameterBinder:            c.parameterBinder,
		ExecutorIntegrationManager: c.executorIntegrationManager,
		ExecutorCredentialManager:  c.executorCredentialManager,
	}), nil
}

type OpenAICredential struct {
	APIKey string `json:"api_key"`
}

type AIAgentExecutor struct {
	integrationSelector        domain.IntegrationSelector
	parameterBinder            domain.IntegrationParameterBinder
	executorIntegrationManager domain.ExecutorIntegrationManager
	actionManager              *domain.IntegrationActionManager
	credentialGetter           domain.CredentialGetter[OpenAICredential]
}

func NewAIAgentExecutor(deps domain.IntegrationDeps) domain.IntegrationExecutor {
	executor := &AIAgentExecutor{
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
	AgentTypeFunctionCalling AgentType = "function_calling"
)

type ExecuteParams struct {
	Prompt       string `json:"prompt,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	MaxSteps     int    `json:"max_steps,omitempty"`
}

type MemoryNodeParams struct {
	SessionID           string `json:"session_id"`
	SessionTTLInSeconds int    `json:"session_ttl_in_seconds"`
	MaxContextLength    int    `json:"max_context_length"`
	ConversationCount   int    `json:"conversation_count"`
}

type LLMNodeParams struct {
	Model       string  `json:"model"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float32 `json:"temperature"`
}

const InputHandleIDFormat = "input-%s-%d"
const OutputHandleIDFormat = "output-%s-%d"

func (e *AIAgentExecutor) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return e.actionManager.Run(ctx, params.ActionType, params)
}

func (e *AIAgentExecutor) ProcessFunctionCalling(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	executeParams := ExecuteParams{}

	err := e.parameterBinder.BindToStruct(ctx, item, &executeParams, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	agentNode, exists := params.Workflow.GetNodeByID(params.NodeID)
	if !exists {
		return nil, fmt.Errorf("agent node %s not found in workflow", params.NodeID)
	}

	subNodes := params.Workflow.GetSubNodes(params.NodeID)

	workflow := *params.Workflow

	executionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return nil, fmt.Errorf("workflow execution context not found")
	}

	agentSettings, err := e.ResolveAgentSettings(ctx, ResolveAgentSettingsParams{
		InputItem:         item,
		AgentNode:         agentNode,
		Workflow:          workflow,
		SubNodes:          subNodes,
		ExecutionObserver: executionContext.ExecutionObserver,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent settings: %w", err)
	}

	initialPrompt := executeParams.Prompt

	if initialPrompt == "" {
		return nil, fmt.Errorf("initial prompt is required")
	}

	llm := agentSettings.LLM
	memory := agentSettings.Memory
	tools := agentSettings.Tools

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

	log.Debug().Interface("tools", tools).Msg("Tools")

	executionObserver := executionContext.ExecutionObserver

	maxSteps := executeParams.MaxSteps
	if maxSteps == 0 {
		maxSteps = 30
	}

	hooksManager := AgentHooksManager{
		AgentNodeID:       agentNode.ID,
		InputItem:         item,
		ExecutionObserver: executionObserver,
		ParameterBinder:   e.parameterBinder,
		LLMNode:           llm.Node,
	}

	hooks := agent.Hooks{
		OnBeforeGenerate:   hooksManager.OnBeforeGenerate,
		OnGenerationFailed: hooksManager.OnGenerationFailed,
		OnStepComplete:     hooksManager.OnStepComplete,
	}

	a, err := agent.New(
		agent.WithModel(llm.LLM),
		agent.WithMaxTokens(4000),
		agent.WithTemperature(1),
		agent.WithSystemPrompt(executeParams.SystemPrompt),
		agent.WithMemory(memory.Memory),
		agent.WithTools(tools...),
		agent.WithMaxIterations(maxSteps),
		agent.WithCancelContext(ctx),
		agent.WithHooks(hooks),
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

	return outputItems, nil
}

type AgentSettings struct {
	LLM    ResolveLLMResult
	Memory ResolveMemoryResult
	Tools  []tool.Tool
}

type ResolveAgentSettingsParams struct {
	InputItem         domain.Item
	AgentNode         domain.WorkflowNode
	Workflow          domain.Workflow
	ExecutionObserver domain.ExecutionObserver
	SubNodes          []domain.WorkflowNode
}

func (e *AIAgentExecutor) ResolveAgentSettings(ctx context.Context, params ResolveAgentSettingsParams) (AgentSettings, error) {
	llm, err := e.ResolveLLM(ctx, params)
	if err != nil {
		return AgentSettings{}, fmt.Errorf("failed to resolve LLM: %w", err)
	}

	memory, err := e.ResolveMemory(ctx, params)
	if err != nil {
		return AgentSettings{}, fmt.Errorf("failed to resolve memory: %w", err)
	}

	tools, err := e.ResolveTools(ctx, params)
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

type OpenAIModelSettings struct {
	Model        string  `json:"model"`
	Temperature  float32 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
	SystemPrompt string  `json:"system_prompt"`
	CredentialID string  `json:"credential_id"`
}

func (e *AIAgentExecutor) ResolveLLM(ctx context.Context, params ResolveAgentSettingsParams) (ResolveLLMResult, error) {
	llmNode := domain.WorkflowNode{}

	for _, subNode := range params.SubNodes {
		if subNode.UsageContext == string(domain.UsageContextLLMProvider) {
			llmNode = subNode
			break
		}
	}

	if llmNode.ID == "" {
		return ResolveLLMResult{}, fmt.Errorf("LLM node is required")
	}

	settings := OpenAIModelSettings{}

	err := e.parameterBinder.BindToStruct(ctx, params.InputItem, &settings, llmNode.IntegrationSettings)
	if err != nil {
		return ResolveLLMResult{}, fmt.Errorf("failed to bind LLM node params: %w", err)
	}

	if settings.CredentialID == "" {
		return ResolveLLMResult{}, fmt.Errorf("credential_id is not found in LLM node %s", llmNode.ID)
	}

	var languageModel provider.LanguageModel

	switch llmNode.IntegrationType {
	case "openai":
		credential, err := e.credentialGetter.GetDecryptedCredential(ctx, settings.CredentialID)
		if err != nil {
			return ResolveLLMResult{}, fmt.Errorf("failed to get credential: %w", err)
		}

		languageModel = openai.NewWithConfig(openai.Config{
			APIKey:      credential.APIKey,
			Model:       settings.Model,
			Temperature: settings.Temperature,
			MaxTokens:   settings.MaxTokens,
			/* 			SystemPrompt: settings.SystemPrompt, */
		})
	default:
		return ResolveLLMResult{}, fmt.Errorf("unsupported LLM node type: %s", llmNode.IntegrationType)
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

func (e *AIAgentExecutor) ResolveMemory(ctx context.Context, params ResolveAgentSettingsParams) (ResolveMemoryResult, error) {
	memoryNode := domain.WorkflowNode{}

	for _, subNode := range params.SubNodes {
		if subNode.UsageContext == string(domain.UsageContextMemoryProvider) {
			memoryNode = subNode
			break
		}
	}

	if memoryNode.ID == "" {
		return ResolveMemoryResult{}, nil
	}

	creator, err := e.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
		IntegrationType: domain.IntegrationType(memoryNode.IntegrationType),
	})
	if err != nil {
		return ResolveMemoryResult{}, fmt.Errorf("failed to resolve memory: %w", err)
	}

	var credentialID string

	credentialIDValue, exists := memoryNode.IntegrationSettings["credential_id"]
	if exists {
		credentialIDString, ok := credentialIDValue.(string)
		if !ok {
			return ResolveMemoryResult{}, fmt.Errorf("credential_id is not a string in memory node %s", memoryNode.ID)
		}

		credentialID = credentialIDString
	}

	memory, err := creator.CreateIntegration(ctx, domain.CreateIntegrationParams{
		WorkspaceID:  params.Workflow.WorkspaceID,
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

func (e *AIAgentExecutor) ResolveTools(ctx context.Context, params ResolveAgentSettingsParams) ([]tool.Tool, error) {
	toolsHandleID := fmt.Sprintf(InputHandleIDFormat, params.AgentNode.ID, 3)

	toolsInput, exists := params.AgentNode.GetInputByID(toolsHandleID)
	if !exists {
		return nil, nil
	}

	log.Debug().Interface("tools_input", toolsInput).Msg("Tools input")

	if len(toolsInput.SubscribedEvents) == 0 {
		log.Debug().Msg("Tools input has no subscribed events")
		return nil, nil
	}

	toolNodeIDs := e.GetNodeIDsFromOutputIDs(toolsInput.SubscribedEvents)

	nodeReferences := make([]NodeReference, 0, len(toolNodeIDs))

	for _, toolNodeID := range toolNodeIDs {
		nodeReferences = append(nodeReferences, NodeReference{NodeID: toolNodeID})
	}

	log.Debug().Interface("node_references", nodeReferences).Msg("Node references")

	if len(nodeReferences) == 0 {
		return nil, nil
	}

	toolCreator := NewIntegrationToolCreator(IntegrationToolCreatorDeps{
		IntegrationSelector:        e.integrationSelector,
		ExecutorIntegrationManager: e.executorIntegrationManager,
		ExecutionObserver:          params.ExecutionObserver,
	})

	tools, err := toolCreator.CreateTools(ctx, CreateToolsParams{
		Workflow:       params.Workflow,
		NodeReferences: nodeReferences,
		AgentNode:      params.AgentNode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tools: %w", err)
	}

	return tools, nil
}

func (e *AIAgentExecutor) GetNodeIDsFromOutputIDs(outputIDs []string) []string {
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

func (e *AIAgentExecutor) GetNodeIDFromOutputID(outputID string) string {
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

type CreateToolsParams struct {
	Workflow       domain.Workflow
	NodeReferences []NodeReference
	AgentNode      domain.WorkflowNode
}

func (c *IntegrationToolCreator) CreateTools(ctx context.Context, params CreateToolsParams) ([]tool.Tool, error) {
	workflow := params.Workflow
	nodeReferences := params.NodeReferences

	toolNodes := make([]domain.WorkflowNode, 0)

	for _, nodeReference := range nodeReferences {
		node, exists := workflow.GetNodeByID(nodeReference.NodeID)
		if !exists {
			return nil, fmt.Errorf("node %s not found in workflow", nodeReference.NodeID)
		}

		if node.IntegrationType == domain.IntegrationType_Toolset {
			toolNodes = append(toolNodes, workflow.GetSubNodes(node.ID)...)

			continue
		}

		toolNodes = append(toolNodes, node)
	}

	log.Debug().Interface("tool_nodes", toolNodes).Msg("Tool nodes")

	tools := make([]tool.Tool, 0, len(toolNodes))

	for _, toolNode := range toolNodes {

		creator, err := c.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
			IntegrationType: domain.IntegrationType(toolNode.IntegrationType),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve tool %s: %w", toolNode.Name, err)
		}

		credentialID, exists := toolNode.IntegrationSettings["credential_id"]
		if !exists {
			credentialID = ""
		}

		credentialIDString, ok := credentialID.(string)
		if !ok {
			return nil, fmt.Errorf("credential_id is not a string in tool node %s", toolNode.ID)
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

		agentToolInputHandleID := fmt.Sprintf(InputHandleIDFormat, params.AgentNode.ID, 1)

		executeFunc := func(args string) (string, error) {
			log.Debug().Str("node_id", toolNode.ID).Msg("Notifying observer about tool execution started")

			err = c.observer.Notify(ctx, executor.NodeExecutionStartedEvent{
				NodeID:    toolNode.ID,
				Timestamp: time.Now(),
			})
			if err != nil {
				return "", fmt.Errorf("ai agent integration failed to notify observer about tool execution started: %w", err)
			}

			var inputItem map[string]any

			err := json.Unmarshal([]byte(args), &inputItem)
			if err != nil {
				return "", fmt.Errorf("failed to unmarshal tool input: %w", err)
			}

			inputItems := []domain.Item{inputItem}

			inputPayload, err := json.Marshal(inputItems)
			if err != nil {
				return "", fmt.Errorf("failed to marshal tool input: %w", err)
			}

			p := domain.IntegrationInput{
				NodeID:     toolNode.ID,
				ActionType: toolNode.ActionNodeOpts.ActionType,
				IntegrationParams: domain.IntegrationParams{
					Settings: inputItem,
				},
				PayloadByInputID: map[string]domain.Payload{
					agentToolInputHandleID: domain.Payload(inputPayload),
				},
				Workflow: &workflow,
			}

			itemsByInputID := map[string]domain.NodeItems{
				agentToolInputHandleID: {
					FromNodeID: params.AgentNode.ID,
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
				IntegrationType:       toolNode.IntegrationType,
				IntegrationActionType: toolNode.ActionNodeOpts.ActionType,
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
	integration, err := c.executorIntegrationManager.GetIntegration(ctx, domain.IntegrationType(toolNode.IntegrationType))
	if err != nil {
		return types.Tool{}, fmt.Errorf("failed to get integration for type %s: %w", toolNode.IntegrationType, err)
	}

	var action domain.IntegrationAction
	found := false
	for _, a := range integration.Actions {
		if a.ActionType == toolNode.ActionNodeOpts.ActionType {
			action = a
			found = true
			break
		}
	}
	if !found {
		return types.Tool{}, fmt.Errorf("action not found for type %s in integration %s", toolNode.ActionNodeOpts.ActionType, toolNode.IntegrationType)
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
	toolName := fmt.Sprintf("%s_%s", strings.ToLower(string(toolNode.IntegrationType)), strings.ToLower(string(action.ActionType)))

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

	// Handle TagInput type - always an array of strings
	if prop.Type == domain.NodePropertyType_TagInput {
		schema["items"] = map[string]any{
			"type": "string",
		}
	}

	// Handle array types
	if prop.Type == domain.NodePropertyType_Array {
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

type PropertySentinelSearcher struct {
	sentinel string
}

func NewPropertySentinelSearcher(sentinel string) *PropertySentinelSearcher {
	return &PropertySentinelSearcher{sentinel: sentinel}
}

func (s *PropertySentinelSearcher) Search(ctx context.Context, value any, property domain.NodeProperty) (JSONSchemaProperty, bool) {
	stringSearcher := StringSentinelSearcher{sentinel: s.sentinel}
	arraySearcher := ArraySentinelSearcher{sentinelSearcher: PropertySentinelSearcher{
		sentinel: s.sentinel,
	}}

	switch property.Type {
	case domain.NodePropertyType_String:
		return stringSearcher.Search(ctx, value, property)
	case domain.NodePropertyType_Array:
		return arraySearcher.Search(ctx, value, property)
	}

	return JSONSchemaProperty{}, false
}

type StringSentinelSearcher struct {
	sentinel string
}

func (s *StringSentinelSearcher) Search(ctx context.Context, value any, property domain.NodeProperty) (JSONSchemaProperty, bool) {
	p := JSONSchemaProperty{
		Type:        "string",
		Description: property.Description,
		Properties:  make(map[string]JSONSchemaProperty),
		Required:    []string{},
		MinLength:   property.MinLength,
		MaxLength:   property.MaxLength,
		Pattern:     property.Pattern,
	}

	valueString, ok := value.(string)
	if !ok {
		return p, false
	}

	if valueString == s.sentinel {
		return p, true
	}

	return p, false
}

type ArraySentinelSearcher struct {
	sentinelSearcher PropertySentinelSearcher
}

func (s *ArraySentinelSearcher) Search(ctx context.Context, value any, property domain.NodeProperty) (JSONSchemaProperty, bool) {
	p := JSONSchemaProperty{
		Type:        "array",
		Description: property.Description,
		Properties:  make(map[string]JSONSchemaProperty),
		Required:    []string{},
		Items: &JSONSchemaProperty{
			Type:       "object",
			Properties: make(map[string]JSONSchemaProperty),
			Required:   []string{},
		},
		MinItems: property.ArrayOpts.MinItems,
		MaxItems: property.ArrayOpts.MaxItems,
	}

	valueArray, ok := value.([]any)
	if !ok {
		return JSONSchemaProperty{}, false
	}

	for _, value := range valueArray {
		for _, itemProperty := range property.ArrayOpts.ItemProperties {
			log.Debug().Interface("value", value).Msg("Value")

			valueMap, ok := value.(map[string]any)
			if !ok {
				continue
			}

			subPropertyValue, exists := valueMap[itemProperty.Key]
			if !exists {
				continue
			}

			subProperty, ok := s.sentinelSearcher.Search(ctx, subPropertyValue, itemProperty)
			if !ok {
				continue
			}

			p.Items.Properties[itemProperty.Key] = subProperty
		}
	}

	if len(p.Items.Properties) == 0 {
		return JSONSchemaProperty{}, false
	}

	return p, true
}

type JSONSchemaProperty struct {
	Type        string                        `json:"type"`
	Description string                        `json:"description,omitempty"`
	Properties  map[string]JSONSchemaProperty `json:"properties,omitempty"`
	Required    []string                      `json:"required,omitempty"`
	Items       *JSONSchemaProperty           `json:"items,omitempty"`
	MinItems    int                           `json:"minItems,omitempty"`
	MaxItems    int                           `json:"maxItems,omitempty"`
	Enum        []string                      `json:"enum,omitempty"`
	MinLength   int                           `json:"minLength,omitempty"`
	MaxLength   int                           `json:"maxLength,omitempty"`
	Pattern     string                        `json:"pattern,omitempty"`
}

type AgentHooksManager struct {
	AgentNodeID       string
	InputItem         domain.Item
	ExecutionObserver domain.ExecutionObserver
	ParameterBinder   domain.IntegrationParameterBinder
	LLMNode           domain.WorkflowNode
}

func (m *AgentHooksManager) OnBeforeGenerate(ctx context.Context, req *provider.GenerateRequest) {
	llmNodeParams := LLMNodeParams{}

	item := m.InputItem

	err := m.ParameterBinder.BindToStruct(ctx, item, &llmNodeParams, m.LLMNode.IntegrationSettings)
	if err != nil {
		log.Error().Err(err).Msg("failed to bind LLM node params")
	}

	req.MaxTokens = llmNodeParams.MaxTokens
	req.Temperature = llmNodeParams.Temperature

	err = m.ExecutionObserver.Notify(ctx, executor.NodeExecutionStartedEvent{
		NodeID:    m.LLMNode.ID,
		Timestamp: time.Now(),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to notify execution observer about LLM node execution started")
	}
}

func (m *AgentHooksManager) OnGenerationFailed(ctx context.Context, req *provider.GenerateRequest, step *agent.Step, generationErr error) {
	log.Error().Err(generationErr).Msg("generation failed")

	llmInputHandleID := fmt.Sprintf(InputHandleIDFormat, m.LLMNode.ID, 0)

	llmInputItem := map[string]any{}

	rawRequestJSON, err := json.Marshal(step.GenerateRequest)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal LLM input item")
	}

	err = json.Unmarshal(rawRequestJSON, &llmInputItem)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal LLM input item")
	}

	item := m.InputItem

	llmInputItem["agent"] = map[string]any{
		"item": item,
	}

	itemsByInputID := map[string]domain.NodeItems{
		llmInputHandleID: {
			FromNodeID: m.AgentNodeID,
			Items:      []domain.Item{llmInputItem},
		},
	}

	now := time.Now()

	err = m.ExecutionObserver.Notify(ctx, executor.NodeExecutionFailedEvent{
		NodeID:         m.LLMNode.ID,
		ItemsByInputID: itemsByInputID,
		Timestamp:      now,
		Error:          generationErr,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to notify execution observer about LLM node execution failed")
	}
}

func (m *AgentHooksManager) OnStepComplete(ctx context.Context, step *agent.Step) {
	item := m.InputItem
	llmNode := m.LLMNode

	llmInputHandleID := fmt.Sprintf(InputHandleIDFormat, llmNode.ID, 0)
	llmOutputHandleID := fmt.Sprintf(OutputHandleIDFormat, llmNode.ID, 0)

	llmInputItem := map[string]any{}

	rawRequestJSON, err := json.Marshal(step.GenerateRequest)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal LLM input item")
	}

	err = json.Unmarshal(rawRequestJSON, &llmInputItem)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal LLM input item")
	}

	llmInputItem["agent"] = map[string]any{
		"item": item,
	}

	itemsByInputID := map[string]domain.NodeItems{
		llmInputHandleID: {
			FromNodeID: m.AgentNodeID,
			Items:      []domain.Item{llmInputItem},
		},
	}

	itemsByOutputID := map[string]domain.NodeItems{
		llmOutputHandleID: {
			FromNodeID: llmNode.ID,
			Items:      []domain.Item{step},
		},
	}

	now := time.Now()

	err = m.ExecutionObserver.Notify(ctx, executor.NodeExecutionCompletedEvent{
		NodeID:                llmNode.ID,
		ItemsByInputID:        itemsByInputID,
		ItemsByOutputID:       itemsByOutputID,
		StartedAt:             now,
		EndedAt:               now,
		IntegrationType:       domain.IntegrationType(llmNode.IntegrationType),
		IntegrationActionType: domain.IntegrationActionType(llmNode.ActionNodeOpts.ActionType),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to notify execution observer about LLM node execution completed")
	}
}

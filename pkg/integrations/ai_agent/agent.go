package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/agent"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider/anthropic"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider/gemini"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider/openai"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/tool"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/flowbaker/flowbaker/pkg/domain/executor"
	"github.com/flowbaker/flowbaker/pkg/integrations/ai_agent/memory/mongodb"
	postgresqlmemory "github.com/flowbaker/flowbaker/pkg/integrations/ai_agent/memory/postgresql"
	redismemory "github.com/flowbaker/flowbaker/pkg/integrations/ai_agent/memory/redis"
	"github.com/flowbaker/flowbaker/pkg/integrations/flowbaker_agent_memory"
	mongodbIntegration "github.com/flowbaker/flowbaker/pkg/integrations/mongo"
	postgresqlIntegration "github.com/flowbaker/flowbaker/pkg/integrations/postgresql"
	redisIntegration "github.com/flowbaker/flowbaker/pkg/integrations/redis"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
)

const (
	IntegrationActionType_ExecuteAgent domain.IntegrationActionType = "execute_agent"
)

type AIAgentCreator struct {
	integrationSelector        domain.IntegrationSelector
	parameterBinder            domain.IntegrationParameterBinder
	executorIntegrationManager domain.ExecutorIntegrationManager
	executorCredentialManager  domain.ExecutorCredentialManager
	flowbakerClient            flowbaker.ClientInterface
}

func NewAIAgentCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &AIAgentCreator{
		integrationSelector:        deps.IntegrationSelector,
		parameterBinder:            deps.ParameterBinder,
		executorIntegrationManager: deps.ExecutorIntegrationManager,
		executorCredentialManager:  deps.ExecutorCredentialManager,
		flowbakerClient:            deps.FlowbakerClient,
	}
}

func (c *AIAgentCreator) CreateIntegration(ctx context.Context, params domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewAIAgentExecutor(domain.IntegrationDeps{
		IntegrationSelector:        c.integrationSelector,
		ParameterBinder:            c.parameterBinder,
		ExecutorIntegrationManager: c.executorIntegrationManager,
		ExecutorCredentialManager:  c.executorCredentialManager,
		FlowbakerClient:            c.flowbakerClient,
	}), nil
}

type OpenAICredential struct {
	APIKey string `json:"api_key"`
}

type AnthropicCredential struct {
	APIKey string `json:"api_key"`
}

type GeminiCredential struct {
	APIKey string `json:"api_key"`
}

type GroqCredential struct {
	APIKey string `json:"api_key"`
}

type AIAgentExecutor struct {
	integrationSelector        domain.IntegrationSelector
	parameterBinder            domain.IntegrationParameterBinder
	executorIntegrationManager domain.ExecutorIntegrationManager
	executorCredentialManager  domain.ExecutorCredentialManager
	actionManager              *domain.IntegrationActionManager
	client                     flowbaker.ClientInterface
}

func NewAIAgentExecutor(deps domain.IntegrationDeps) domain.IntegrationExecutor {
	executor := &AIAgentExecutor{
		integrationSelector:        deps.IntegrationSelector,
		parameterBinder:            deps.ParameterBinder,
		executorIntegrationManager: deps.ExecutorIntegrationManager,
		executorCredentialManager:  deps.ExecutorCredentialManager,
		client:                     deps.FlowbakerClient,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItemMulti(IntegrationActionType_ExecuteAgent, executor.ExecuteAgent)

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

const InputHandleIDFormat = "input-%s-%d"
const OutputHandleIDFormat = "output-%s-%d"

func (e *AIAgentExecutor) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return e.actionManager.Run(ctx, params.ActionType, params)
}

type AIChatStreamEvent struct {
	SessionID         string `json:"session_id"`
	WorkspaceID       string `json:"workspace_id"`
	UserID            string `json:"user_id"`
	EventType         string `json:"event_type"`
	EventData         any    `json:"event_data"`
	Timestamp         int64  `json:"timestamp"`
	IsFromChatTrigger bool   `json:"is_from_chat_trigger"`
}

func (e AIChatStreamEvent) GetEventType() domain.StreamEventType {
	return domain.StreamEventTypeAIChatStream
}

func (e *AIAgentExecutor) ExecuteAgent(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
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
		MemoryNode:        memory.Node,
	}

	hooks := agent.Hooks{
		OnBeforeGenerate:        hooksManager.OnBeforeGenerate,
		OnGenerationFailed:      hooksManager.OnGenerationFailed,
		OnStepComplete:          hooksManager.OnStepComplete,
		OnBeforeMemoryRetrieve:  hooksManager.OnBeforeMemoryRetrieve,
		OnMemoryRetrieved:       hooksManager.OnMemoryRetrieved,
		OnMemoryRetrievalFailed: hooksManager.OnMemoryRetrievalFailed,
		OnBeforeMemorySave:      hooksManager.OnBeforeMemorySave,
		OnMemorySaved:           hooksManager.OnMemorySaved,
		OnMemorySaveFailed:      hooksManager.OnMemorySaveFailed,
	}

	a, err := agent.New(
		agent.WithModel(llm.LLM),
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

	hooksManager.Agent = a

	result, err := a.Chat(ctx, agent.ChatRequest{
		Prompt:    initialPrompt,
		SessionID: agentSettings.Memory.SessionID,
	})
	if err != nil {
		log.Error().Str("err", err.Error()).Msg("failed to chat with agent")
		return nil, fmt.Errorf("failed to chat with agent: %w", err)
	}

	wg := sync.WaitGroup{}

	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-result.EventChan:
				if !ok {
					return
				}
				eventType := string(event.GetType())

				disabledEventTypes := []string{
					"tool-call-start",
					"tool-call-delta",
					"tool-call-complete",
					"tool-execution-start",
					"tool-execution-complete",
				}

				if slices.Contains(disabledEventTypes, eventType) {
					continue
				}

				streamEvent := AIChatStreamEvent{
					SessionID:         agentSettings.Memory.SessionID,
					WorkspaceID:       executionContext.WorkspaceID,
					EventType:         string(event.GetType()),
					EventData:         event,
					Timestamp:         time.Now().Unix(),
					IsFromChatTrigger: true,
				}

				if executionContext.UserID != nil {
					streamEvent.UserID = *executionContext.UserID
				}

				err = executionObserver.NotifyStream(ctx, streamEvent)
				if err != nil {
					log.Error().Str("err", err.Error()).Msg("failed to notify stream event")
				}

			case err := <-result.ErrChan:
				if err != nil {
					log.Error().Str("err", err.Error()).Msg("failed to chat with agent")
				}
				return
			}
		}
	})

	wg.Wait()

	maxStepNumber := math.MinInt
	maxStep := &agent.Step{}

	steps := a.GetSteps()

	for _, step := range steps {
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
	LLM       provider.LanguageModel
	Node      domain.WorkflowNode
	SessionID string
}

type ModelSettings struct {
	Model            string   `json:"model"`
	Temperature      float32  `json:"temperature"`
	MaxTokens        int      `json:"max_tokens"`
	SystemPrompt     string   `json:"system_prompt"`
	CredentialID     string   `json:"credential_id"`
	TopP             float32  `json:"top_p"`
	FrequencyPenalty float32  `json:"frequency_penalty"`
	PresencePenalty  float32  `json:"presence_penalty"`
	Stop             []string `json:"stop"`
	ReasoningEffort  string   `json:"reasoning_effort"`
	Verbosity        string   `json:"verbosity"`
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

	settings := ModelSettings{}

	err := e.parameterBinder.BindToStruct(ctx, params.InputItem, &settings, llmNode.IntegrationSettings)
	if err != nil {
		return ResolveLLMResult{}, fmt.Errorf("failed to bind LLM node params: %w", err)
	}

	if settings.CredentialID == "" {
		return ResolveLLMResult{}, fmt.Errorf("credential_id is not found in LLM node %s", llmNode.ID)
	}

	var languageModel provider.LanguageModel

	switch llmNode.IntegrationType {
	case domain.IntegrationType_OpenAI:
		credentialGetter := managers.NewExecutorCredentialGetter[OpenAICredential](e.executorCredentialManager)

		credential, err := credentialGetter.GetDecryptedCredential(ctx, settings.CredentialID)
		if err != nil {
			return ResolveLLMResult{}, fmt.Errorf("failed to get credential: %w", err)
		}

		openaiModel := openai.New(credential.APIKey, settings.Model)

		openaiModel.SetRequestSettings(openai.RequestSettings{
			Model:            settings.Model,
			Temperature:      settings.Temperature,
			MaxTokens:        settings.MaxTokens,
			TopP:             settings.TopP,
			FrequencyPenalty: settings.FrequencyPenalty,
			PresencePenalty:  settings.PresencePenalty,
			Stop:             settings.Stop,
			ReasoningEffort:  settings.ReasoningEffort,
			Verbosity:        settings.Verbosity,
		})

		languageModel = openaiModel

	case domain.IntegrationType_Anthropic:
		credentialGetter := managers.NewExecutorCredentialGetter[AnthropicCredential](e.executorCredentialManager)

		credential, err := credentialGetter.GetDecryptedCredential(ctx, settings.CredentialID)
		if err != nil {
			return ResolveLLMResult{}, fmt.Errorf("failed to get credential: %w", err)
		}

		anthropicModel := anthropic.New(credential.APIKey, settings.Model)

		languageModel = anthropicModel

	case domain.IntegrationType_Gemini:
		credentialGetter := managers.NewExecutorCredentialGetter[GeminiCredential](e.executorCredentialManager)

		credential, err := credentialGetter.GetDecryptedCredential(ctx, settings.CredentialID)
		if err != nil {
			return ResolveLLMResult{}, fmt.Errorf("failed to get credential: %w", err)
		}

		geminiModel, err := gemini.New(ctx, credential.APIKey, settings.Model)
		if err != nil {
			return ResolveLLMResult{}, fmt.Errorf("failed to create Gemini provider: %w", err)
		}

		geminiModel.SetRequestSettings(gemini.RequestSettings{
			Model:           settings.Model,
			Temperature:     settings.Temperature,
			MaxOutputTokens: int32(settings.MaxTokens),
			TopP:            settings.TopP,
		})

		languageModel = geminiModel

	case domain.IntegrationType_Groq:
		credentialGetter := managers.NewExecutorCredentialGetter[GroqCredential](e.executorCredentialManager)

		credential, err := credentialGetter.GetDecryptedCredential(ctx, settings.CredentialID)
		if err != nil {
			return ResolveLLMResult{}, fmt.Errorf("failed to get credential: %w", err)
		}

		groqModel := openai.New(
			credential.APIKey,
			settings.Model,
			openai.WithBaseURL("https://api.groq.com/openai/v1"),
		)

		groqModel.SetRequestSettings(openai.RequestSettings{
			Model:       settings.Model,
			Temperature: settings.Temperature,
			MaxTokens:   settings.MaxTokens,
			TopP:        settings.TopP,
		})

		languageModel = groqModel

	default:
		return ResolveLLMResult{}, fmt.Errorf("unsupported LLM node type: %s", llmNode.IntegrationType)
	}

	return ResolveLLMResult{
		LLM:  languageModel,
		Node: llmNode,
	}, nil
}

type ResolveMemoryResult struct {
	Memory    memory.Store
	Node      domain.WorkflowNode
	SessionID string
}

type MongoDBMemoryParams struct {
	Database   string `json:"database"`
	Collection string `json:"collection"`
	SessionID  string `json:"session_id"`
}

type RedisMemoryParams struct {
	KeyPrefix string `json:"key_prefix"`
	SessionID string `json:"session_id"`
}

type PostgreSQLMemoryParams struct {
	TablePrefix string `json:"table_prefix"`
	SessionID   string `json:"session_id"`
}

type FlowbakerAgentMemoryParams struct {
	SessionID string `json:"session_id"`
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

	var credentialID string

	credentialIDValue, exists := memoryNode.IntegrationSettings["credential_id"]
	if exists {
		credentialIDString, ok := credentialIDValue.(string)
		if !ok {
			return ResolveMemoryResult{}, fmt.Errorf("credential_id is not a string in memory node %s", memoryNode.ID)
		}

		credentialID = credentialIDString
	}

	switch memoryNode.IntegrationType {
	case domain.IntegrationType_MongoDB:
		p := MongoDBMemoryParams{}

		err := e.parameterBinder.BindToStruct(ctx, params.InputItem, &p, memoryNode.IntegrationSettings)
		if err != nil {
			return ResolveMemoryResult{}, fmt.Errorf("failed to bind MongoDB memory node params: %w", err)
		}

		deps := mongodb.StoreDeps{
			Context:          ctx,
			CredentialGetter: managers.NewExecutorCredentialGetter[mongodbIntegration.MongoDBCredential](e.executorCredentialManager),
		}

		memory, err := mongodb.New(deps, mongodb.Opts{
			CredentialID:   credentialID,
			DatabaseName:   p.Database,
			CollectionName: p.Collection,
		})
		if err != nil {
			return ResolveMemoryResult{}, fmt.Errorf("failed to create memory: %w", err)
		}

		return ResolveMemoryResult{
			Memory:    memory,
			Node:      memoryNode,
			SessionID: p.SessionID,
		}, nil
	case domain.IntegrationType_Redis:
		p := RedisMemoryParams{}

		err := e.parameterBinder.BindToStruct(ctx, params.InputItem, &p, memoryNode.IntegrationSettings)
		if err != nil {
			return ResolveMemoryResult{}, fmt.Errorf("failed to bind Redis memory node params: %w", err)
		}

		deps := redismemory.StoreDeps{
			Context:          ctx,
			CredentialGetter: managers.NewExecutorCredentialGetter[redisIntegration.RedisCredential](e.executorCredentialManager),
		}

		memory, err := redismemory.New(deps, redismemory.Opts{
			CredentialID: credentialID,
			KeyPrefix:    p.KeyPrefix,
		})
		if err != nil {
			return ResolveMemoryResult{}, fmt.Errorf("failed to create Redis memory: %w", err)
		}

		return ResolveMemoryResult{
			Memory:    memory,
			Node:      memoryNode,
			SessionID: p.SessionID,
		}, nil
	case domain.IntegrationType_PostgreSQL:
		p := PostgreSQLMemoryParams{}

		err := e.parameterBinder.BindToStruct(ctx, params.InputItem, &p, memoryNode.IntegrationSettings)
		if err != nil {
			return ResolveMemoryResult{}, fmt.Errorf("failed to bind PostgreSQL memory node params: %w", err)
		}

		deps := postgresqlmemory.StoreDeps{
			Context:          ctx,
			CredentialGetter: managers.NewExecutorCredentialGetter[postgresqlIntegration.PostgreSQLCredential](e.executorCredentialManager),
		}

		memory, err := postgresqlmemory.New(deps, postgresqlmemory.Opts{
			CredentialID: credentialID,
			TablePrefix:  p.TablePrefix,
		})
		if err != nil {
			return ResolveMemoryResult{}, fmt.Errorf("failed to create PostgreSQL memory: %w", err)
		}

		return ResolveMemoryResult{
			Memory:    memory,
			Node:      memoryNode,
			SessionID: p.SessionID,
		}, nil
	case domain.IntegrationType_FlowbakerAgentMemory:
		p := FlowbakerAgentMemoryParams{}

		err := e.parameterBinder.BindToStruct(ctx, params.InputItem, &p, memoryNode.IntegrationSettings)
		if err != nil {
			return ResolveMemoryResult{}, fmt.Errorf("failed to bind Flowbaker agent memory node params: %w", err)
		}

		memory, err := flowbaker_agent_memory.New(ctx, flowbaker_agent_memory.MemoryDependencies{
			Client:      e.client,
			WorkspaceID: params.Workflow.WorkspaceID,
			Binder:      e.parameterBinder,
		})
		if err != nil {
			return ResolveMemoryResult{}, fmt.Errorf("failed to create memory: %w", err)
		}

		return ResolveMemoryResult{
			Memory:    memory,
			Node:      memoryNode,
			SessionID: p.SessionID,
		}, nil
	}

	return ResolveMemoryResult{}, fmt.Errorf("unsupported memory node type: %s", memoryNode.IntegrationType)
}

func (e *AIAgentExecutor) ResolveTools(ctx context.Context, params ResolveAgentSettingsParams) ([]tool.Tool, error) {
	toolsHandleID := fmt.Sprintf(InputHandleIDFormat, params.AgentNode.ID, 3)

	toolsInput, exists := params.AgentNode.GetInputByID(toolsHandleID)
	if !exists {
		return nil, nil
	}

	if len(toolsInput.SubscribedEvents) == 0 {
		return nil, nil
	}

	toolNodeIDs := e.GetNodeIDsFromOutputIDs(toolsInput.SubscribedEvents)

	nodeReferences := make([]NodeReference, 0, len(toolNodeIDs))

	for _, toolNodeID := range toolNodeIDs {
		nodeReferences = append(nodeReferences, NodeReference{NodeID: toolNodeID})
	}

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

func (c *IntegrationToolCreator) CreateToolSchema(
	ctx context.Context,
	providedByAgent []string,
	properties []domain.NodeProperty,
) map[string]any {
	builder := NewPathBasedSchemaBuilder()
	return builder.Build(providedByAgent, properties)
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

	tools := make([]tool.Tool, 0, len(toolNodes))

	for _, toolNode := range toolNodes {
		integration, err := c.executorIntegrationManager.GetIntegration(ctx, domain.IntegrationType(toolNode.IntegrationType))
		if err != nil {
			return nil, fmt.Errorf("failed to get integration for type %s: %w", toolNode.IntegrationType, err)
		}

		creator, err := c.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
			IntegrationType: domain.IntegrationType(toolNode.IntegrationType),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve tool %s: %w", toolNode.Name, err)
		}

		action, exists := integration.GetActionByType(toolNode.ActionNodeOpts.ActionType)
		if !exists {
			return nil, fmt.Errorf("action %s not found in integration %s", toolNode.ActionNodeOpts.ActionType, integration.Name)
		}

		isCredentialRequired := len(integration.CredentialProperties) > 0 || !integration.IsCredentialOptional

		credentialID, exists := toolNode.IntegrationSettings["credential_id"]
		if !exists {
			if isCredentialRequired {
				return nil, fmt.Errorf("credential is required for tool %s %s, but not provided", integration.Name, action.Name)
			}

			credentialID = ""
		}

		credentialIDString, ok := credentialID.(string)
		if !ok {
			return nil, fmt.Errorf("credential_id is not a string in tool node %s", toolNode.ID)
		}

		delete(toolNode.IntegrationSettings, "credential_id")

		integrationExecutor, err := creator.CreateIntegration(ctx, domain.CreateIntegrationParams{
			WorkspaceID:  workflow.WorkspaceID,
			CredentialID: credentialIDString,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create tool %s: %w", toolNode.Name, err)
		}

		toolParameters := c.CreateToolSchema(ctx, toolNode.ProvidedByAgent, action.Properties)
		toolName := fmt.Sprintf("%s_%s", strings.ToLower(string(toolNode.IntegrationType)), strings.ToLower(string(action.ActionType)))
		toolDescription := fmt.Sprintf("%s: %s", action.Name, action.Description)

		agentToolInputHandleID := fmt.Sprintf(InputHandleIDFormat, params.AgentNode.ID, 1)

		executeFunc := func(args string) (string, error) {
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

			merger := NewPathBasedSettingsMerger()
			mergedSettings := merger.Merge(toolNode.IntegrationSettings, toolNode.ProvidedByAgent, inputItem)

			inputItems := []domain.Item{mergedSettings}

			inputPayload, err := json.Marshal(inputItems)
			if err != nil {
				return "", fmt.Errorf("failed to marshal tool input: %w", err)
			}

			p := domain.IntegrationInput{
				NodeID:     toolNode.ID,
				ActionType: toolNode.ActionNodeOpts.ActionType,
				IntegrationParams: domain.IntegrationParams{
					Settings: mergedSettings,
				},
				PayloadByInputID: map[string]domain.Payload{
					agentToolInputHandleID: domain.Payload(inputPayload),
				},
				Workflow: &workflow,
			}

			itemsByInputID := map[string]domain.NodeItems{
				agentToolInputHandleID: {
					FromNodeID: params.AgentNode.ID,
					Items:      []domain.Item{mergedSettings},
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

		funcTool := tool.Define(toolName, toolDescription, toolParameters, executeFunc)

		log.Debug().Interface("toolname", toolName).Interface("tooldescription", toolDescription).Interface("toolparameters", toolParameters).Msg("Created tool")

		tools = append(tools, funcTool)
	}

	return tools, nil
}

type PropertyPathLookup struct {
	pathManager *domain.PropertyPathManager
}

func NewPropertyPathLookup() *PropertyPathLookup {
	return &PropertyPathLookup{
		pathManager: domain.NewPropertyPathManager(),
	}
}

func (l *PropertyPathLookup) LookupByPath(path string, properties []domain.NodeProperty) (domain.NodeProperty, bool) {
	segments, err := l.pathManager.ParsePath(path)
	if err != nil {
		log.Warn().Str("path", path).Err(err).Msg("failed to parse property path")
		return domain.NodeProperty{}, false
	}

	if len(segments) == 0 {
		return domain.NodeProperty{}, false
	}

	return l.lookupSegments(segments, properties)
}

func (l *PropertyPathLookup) lookupSegments(segments []domain.PropertyPathSegment, properties []domain.NodeProperty) (domain.NodeProperty, bool) {
	if len(segments) == 0 || len(properties) == 0 {
		return domain.NodeProperty{}, false
	}

	currentSegment := segments[0]
	remainingSegments := segments[1:]

	var foundProperty domain.NodeProperty
	found := false
	for _, prop := range properties {
		if prop.Key == currentSegment.Key {
			foundProperty = prop
			found = true
			break
		}
	}

	if !found {
		return domain.NodeProperty{}, false
	}

	if len(remainingSegments) == 0 {
		return foundProperty, true
	}

	childProperties := l.getChildProperties(foundProperty, currentSegment)
	if len(childProperties) == 0 {
		return domain.NodeProperty{}, false
	}

	return l.lookupSegments(remainingSegments, childProperties)
}

func (l *PropertyPathLookup) getChildProperties(prop domain.NodeProperty, segment domain.PropertyPathSegment) []domain.NodeProperty {
	if segment.Index != nil {
		if prop.Type == domain.NodePropertyType_Array && prop.ArrayOpts != nil {
			return prop.ArrayOpts.ItemProperties
		}
		return nil
	}

	switch prop.Type {
	case domain.NodePropertyType_Array:
		if prop.ArrayOpts != nil {
			return prop.ArrayOpts.ItemProperties
		}
	case domain.NodePropertyType_Map:
		if prop.MapOpts != nil {
			return prop.MapOpts.Properties
		}
	default:
		if len(prop.SubNodeProperties) > 0 {
			return prop.SubNodeProperties
		}
	}

	return nil
}

type PropertySchemaCreator interface {
	Create(property domain.NodeProperty) JSONSchemaProperty
	Supports(propertyType domain.NodePropertyType) bool
}

type StringSchemaCreator struct{}

func (c *StringSchemaCreator) Supports(propertyType domain.NodePropertyType) bool {
	switch propertyType {
	case domain.NodePropertyType_String,
		domain.NodePropertyType_Text,
		domain.NodePropertyType_CodeEditor,
		domain.NodePropertyType_Date,
		domain.NodePropertyType_Query,
		domain.NodePropertyType_Endpoint:
		return true
	}
	return false
}

func (c *StringSchemaCreator) Create(property domain.NodeProperty) JSONSchemaProperty {
	return JSONSchemaProperty{
		Type:        "string",
		Description: property.Description,
		MinLength:   property.MinLength,
		MaxLength:   property.MaxLength,
		Pattern:     property.Pattern,
	}
}

type NumberSchemaCreator struct{}

func (c *NumberSchemaCreator) Supports(propertyType domain.NodePropertyType) bool {
	switch propertyType {
	case domain.NodePropertyType_Integer,
		domain.NodePropertyType_Number,
		domain.NodePropertyType_Float:
		return true
	}
	return false
}

func (c *NumberSchemaCreator) Create(property domain.NodeProperty) JSONSchemaProperty {
	schemaType := "number"
	if property.Type == domain.NodePropertyType_Integer {
		schemaType = "integer"
	}
	return JSONSchemaProperty{
		Type:        schemaType,
		Description: property.Description,
	}
}

type BooleanSchemaCreator struct{}

func (c *BooleanSchemaCreator) Supports(propertyType domain.NodePropertyType) bool {
	return propertyType == domain.NodePropertyType_Boolean
}

func (c *BooleanSchemaCreator) Create(property domain.NodeProperty) JSONSchemaProperty {
	return JSONSchemaProperty{
		Type:        "boolean",
		Description: property.Description,
	}
}

type ArraySchemaCreator struct{}

func (c *ArraySchemaCreator) Supports(propertyType domain.NodePropertyType) bool {
	switch propertyType {
	case domain.NodePropertyType_Array,
		domain.NodePropertyType_TagInput,
		domain.NodePropertyType_ListTagInput:
		return true
	}
	return false
}

func (c *ArraySchemaCreator) Create(property domain.NodeProperty) JSONSchemaProperty {
	schema := JSONSchemaProperty{
		Type:        "array",
		Description: property.Description,
	}

	if property.ArrayOpts != nil {
		schema.MinItems = property.ArrayOpts.MinItems
		schema.MaxItems = property.ArrayOpts.MaxItems
	}

	switch property.Type {
	case domain.NodePropertyType_TagInput:
		schema.Items = &JSONSchemaProperty{Type: "string"}
	case domain.NodePropertyType_ListTagInput:
		schema.Items = &JSONSchemaProperty{
			Type:  "array",
			Items: &JSONSchemaProperty{Type: "string"},
		}
	default:
		schema.Items = &JSONSchemaProperty{Type: "object"}
	}

	return schema
}

type MapSchemaCreator struct{}

func (c *MapSchemaCreator) Supports(propertyType domain.NodePropertyType) bool {
	return propertyType == domain.NodePropertyType_Map
}

func (c *MapSchemaCreator) Create(property domain.NodeProperty) JSONSchemaProperty {
	return JSONSchemaProperty{
		Type:        "object",
		Description: property.Description,
	}
}

type PathBasedSchemaBuilder struct {
	lookup   *PropertyPathLookup
	creators []PropertySchemaCreator
}

func NewPathBasedSchemaBuilder() *PathBasedSchemaBuilder {
	return &PathBasedSchemaBuilder{
		lookup: NewPropertyPathLookup(),
		creators: []PropertySchemaCreator{
			&StringSchemaCreator{},
			&NumberSchemaCreator{},
			&BooleanSchemaCreator{},
			&ArraySchemaCreator{},
			&MapSchemaCreator{},
		},
	}
}

func (b *PathBasedSchemaBuilder) Build(paths []string, properties []domain.NodeProperty) map[string]any {
	schemaProperties := make(map[string]any)
	required := []string{}

	for _, path := range paths {
		prop, found := b.lookup.LookupByPath(path, properties)
		if !found {
			log.Warn().Str("path", path).Msg("property not found for path, skipping")
			continue
		}

		schema := b.createSchemaForProperty(prop)
		schemaProperties[path] = schema

		if prop.Required {
			required = append(required, path)
		}
	}

	result := map[string]any{
		"type":       "object",
		"properties": schemaProperties,
	}

	if len(required) > 0 {
		result["required"] = required
	}

	return result
}

func (b *PathBasedSchemaBuilder) createSchemaForProperty(property domain.NodeProperty) JSONSchemaProperty {
	for _, creator := range b.creators {
		if creator.Supports(property.Type) {
			return creator.Create(property)
		}
	}

	return JSONSchemaProperty{
		Type:        "string",
		Description: property.Description,
	}
}

type PathBasedSettingsMerger struct {
	pathManager *domain.PropertyPathManager
}

func NewPathBasedSettingsMerger() *PathBasedSettingsMerger {
	return &PathBasedSettingsMerger{
		pathManager: domain.NewPropertyPathManager(),
	}
}

func (m *PathBasedSettingsMerger) Merge(
	userSettings map[string]any,
	providedByAgent []string,
	aiValues map[string]any,
) map[string]any {
	result := m.deepCopy(userSettings).(map[string]any)

	for _, path := range providedByAgent {
		aiValue, exists := aiValues[path]
		if !exists {
			continue
		}

		err := m.SetValueAtPath(result, path, aiValue)
		if err != nil {
			log.Warn().Str("path", path).Err(err).Msg("failed to set value at path")
		}
	}

	return result
}

func (m *PathBasedSettingsMerger) SetValueAtPath(target map[string]any, path string, value any) error {
	segments, err := m.pathManager.ParsePath(path)
	if err != nil {
		return fmt.Errorf("failed to parse path: %w", err)
	}

	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	return m.setValueAtSegments(target, segments, value)
}

func (m *PathBasedSettingsMerger) setValueAtSegments(target any, segments []domain.PropertyPathSegment, value any) error {
	if len(segments) == 0 {
		return nil
	}

	segment := segments[0]
	isLastSegment := len(segments) == 1

	switch t := target.(type) {
	case map[string]any:
		return m.setInMap(t, segment, segments[1:], value, isLastSegment)
	case []any:
		return m.setInArray(t, segment, segments[1:], value, isLastSegment)
	default:
		return fmt.Errorf("unexpected target type: %T", target)
	}
}

func (m *PathBasedSettingsMerger) setInMap(target map[string]any, segment domain.PropertyPathSegment, remainingSegments []domain.PropertyPathSegment, value any, isLastSegment bool) error {
	key := segment.Key

	if segment.Index != nil {
		existing, exists := target[key]
		var arr []any
		if exists {
			if a, ok := existing.([]any); ok {
				arr = a
			} else {
				arr = []any{}
			}
		} else {
			arr = []any{}
		}

		arr = m.ensureArrayIndex(arr, *segment.Index)
		target[key] = arr

		if isLastSegment {
			arr = append(arr, value)
			target[key] = arr
			return nil
		}

		if arr[*segment.Index] == nil {
			arr[*segment.Index] = make(map[string]any)
		}
		return m.setValueAtSegments(arr[*segment.Index], remainingSegments, value)
	}

	if isLastSegment {
		target[key] = value
		return nil
	}

	existing, exists := target[key]
	if !exists {
		if len(remainingSegments) > 0 && remainingSegments[0].Index != nil {
			target[key] = []any{}
		} else {
			target[key] = make(map[string]any)
		}
		existing = target[key]
	}

	return m.setValueAtSegments(existing, remainingSegments, value)
}

func (m *PathBasedSettingsMerger) setInArray(target []any, segment domain.PropertyPathSegment, remainingSegments []domain.PropertyPathSegment, value any, isLastSegment bool) error {
	if segment.Index == nil {
		return fmt.Errorf("expected array index for segment %s", segment.Key)
	}

	index := *segment.Index
	target = m.ensureArrayIndex(target, index)

	if isLastSegment {
		target[index] = value
		return nil
	}

	if target[index] == nil {
		target[index] = make(map[string]any)
	}

	return m.setValueAtSegments(target[index], remainingSegments, value)
}

func (m *PathBasedSettingsMerger) ensureArrayIndex(arr []any, index int) []any {
	for len(arr) <= index {
		arr = append(arr, make(map[string]any))
	}
	return arr
}

func (m *PathBasedSettingsMerger) deepCopy(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			result[k] = m.deepCopy(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = m.deepCopy(v)
		}
		return result
	default:
		return v
	}
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
	Agent             *agent.Agent
	InputItem         domain.Item
	ExecutionObserver domain.ExecutionObserver
	ParameterBinder   domain.IntegrationParameterBinder
	LLMNode           domain.WorkflowNode
	MemoryNode        domain.WorkflowNode
}

func (m *AgentHooksManager) OnBeforeGenerate(ctx context.Context, req *provider.GenerateRequest, step *agent.Step) {
	err := m.ExecutionObserver.Notify(ctx, executor.NodeExecutionStartedEvent{
		NodeID:    m.LLMNode.ID,
		Timestamp: time.Now(),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to notify execution observer about LLM node execution started")
	}
}

func (m *AgentHooksManager) OnGenerationFailed(ctx context.Context, req *provider.GenerateRequest, step *agent.Step, generationErr error) {
	llmInputHandleID := fmt.Sprintf(InputHandleIDFormat, m.LLMNode.ID, 0)

	itemsByInputID := map[string]domain.NodeItems{
		llmInputHandleID: {
			FromNodeID: m.AgentNodeID,
			Items:      []domain.Item{m.InputItem},
		},
	}

	now := time.Now()

	err := m.ExecutionObserver.Notify(ctx, executor.NodeExecutionFailedEvent{
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

	itemsByInputID := map[string]domain.NodeItems{
		llmInputHandleID: {
			FromNodeID: m.AgentNodeID,
			Items:      []domain.Item{item},
		},
	}

	itemsByOutputID := map[string]domain.NodeItems{
		llmOutputHandleID: {
			FromNodeID: llmNode.ID,
			Items:      []domain.Item{step},
		},
	}

	now := time.Now()

	err := m.ExecutionObserver.Notify(ctx, executor.NodeExecutionCompletedEvent{
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

func (m *AgentHooksManager) OnBeforeMemoryRetrieve(ctx context.Context, filter memory.Filter) {
	memoryNode := m.MemoryNode

	now := time.Now()

	err := m.ExecutionObserver.Notify(ctx, executor.NodeExecutionStartedEvent{
		NodeID:    memoryNode.ID,
		Timestamp: now,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to notify execution observer about memory node execution started")
	}
}

func (m *AgentHooksManager) OnMemoryRetrieved(ctx context.Context, filter memory.Filter, conversation types.Conversation) {
	item := m.InputItem
	memoryNode := m.MemoryNode

	memoryInputHandleID := fmt.Sprintf(InputHandleIDFormat, memoryNode.ID, 0)
	memoryOutputHandleID := fmt.Sprintf(OutputHandleIDFormat, memoryNode.ID, 0)

	itemsByInputID := map[string]domain.NodeItems{
		memoryInputHandleID: {
			FromNodeID: m.AgentNodeID,
			Items:      []domain.Item{item},
		},
	}

	outputItems := []domain.Item{
		conversation,
	}

	itemsByOutputID := map[string]domain.NodeItems{
		memoryOutputHandleID: {
			FromNodeID: memoryNode.ID,
			Items:      outputItems,
		},
	}

	now := time.Now()

	err := m.ExecutionObserver.Notify(ctx, executor.NodeExecutionCompletedEvent{
		NodeID:                memoryNode.ID,
		ItemsByInputID:        itemsByInputID,
		ItemsByOutputID:       itemsByOutputID,
		StartedAt:             now,
		EndedAt:               now,
		IntegrationType:       domain.IntegrationType(memoryNode.IntegrationType),
		IntegrationActionType: domain.IntegrationActionType(memoryNode.ActionNodeOpts.ActionType),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to notify execution observer about memory node execution completed")
	}
}

func (m *AgentHooksManager) OnMemoryRetrievalFailed(ctx context.Context, filter memory.Filter, err error) {
	item := m.InputItem
	memoryNode := m.MemoryNode

	memoryInputHandleID := fmt.Sprintf(InputHandleIDFormat, memoryNode.ID, 0)

	now := time.Now()

	itemsByInputID := map[string]domain.NodeItems{
		memoryInputHandleID: {
			FromNodeID: m.AgentNodeID,
			Items:      []domain.Item{item},
		},
	}

	err = m.ExecutionObserver.Notify(ctx, executor.NodeExecutionFailedEvent{
		NodeID:         memoryNode.ID,
		ItemsByInputID: itemsByInputID,
		Timestamp:      now,
		Error:          err,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to notify execution observer about memory node execution failed")
	}
}

func (m *AgentHooksManager) OnBeforeMemorySave(ctx context.Context, conversation types.Conversation) {
	memoryNode := m.MemoryNode

	now := time.Now()

	err := m.ExecutionObserver.Notify(ctx, executor.NodeExecutionStartedEvent{
		NodeID:    memoryNode.ID,
		Timestamp: now,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to notify execution observer about memory node execution started")
	}
}

func (m *AgentHooksManager) OnMemorySaveFailed(ctx context.Context, conversation types.Conversation, err error) {
	memoryNode := m.MemoryNode

	now := time.Now()

	memoryInputHandleID := fmt.Sprintf(InputHandleIDFormat, memoryNode.ID, 0)

	itemsByInputID := map[string]domain.NodeItems{
		memoryInputHandleID: {
			FromNodeID: m.AgentNodeID,
			Items:      []domain.Item{m.InputItem},
		},
	}

	err = m.ExecutionObserver.Notify(ctx, executor.NodeExecutionFailedEvent{
		NodeID:         memoryNode.ID,
		ItemsByInputID: itemsByInputID,
		Timestamp:      now,
		Error:          err,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to notify execution observer about memory node execution failed")
	}
}

func (m *AgentHooksManager) OnMemorySaved(ctx context.Context, conversation types.Conversation) {
	memoryNode := m.MemoryNode

	memoryInputHandleID := fmt.Sprintf(InputHandleIDFormat, memoryNode.ID, 0)
	memoryOutputHandleID := fmt.Sprintf(OutputHandleIDFormat, memoryNode.ID, 0)

	itemsByInputID := map[string]domain.NodeItems{
		memoryInputHandleID: {
			FromNodeID: m.AgentNodeID,
			Items:      []domain.Item{m.InputItem},
		},
	}

	itemsByOutputID := map[string]domain.NodeItems{
		memoryOutputHandleID: {
			FromNodeID: memoryNode.ID,
			Items:      []domain.Item{conversation},
		},
	}

	now := time.Now()

	err := m.ExecutionObserver.Notify(ctx, executor.NodeExecutionCompletedEvent{
		NodeID:                memoryNode.ID,
		ItemsByInputID:        itemsByInputID,
		ItemsByOutputID:       itemsByOutputID,
		StartedAt:             now,
		EndedAt:               now,
		IntegrationType:       domain.IntegrationType(memoryNode.IntegrationType),
		IntegrationActionType: domain.IntegrationActionType(memoryNode.ActionNodeOpts.ActionType),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to notify execution observer about memory node execution completed")
	}
}

package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"

	"github.com/flowbaker/flowbaker/pkg/domain/mappers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
)

type NodeExecutionResult struct {
	Output                domain.IntegrationOutput
	IntegrationType       domain.IntegrationType
	IntegrationActionType domain.IntegrationActionType
}

type WorkflowExecutorI interface {
	Execute(ctx context.Context, nodeID string, items []domain.Item) (ExecutionResult, error)
}

type WorkflowExecutor struct {
	executionID string
	workflow    domain.Workflow

	waitingExecutionTasks []WaitingExecutionTask
	executionQueue        []NodeExecutionTask
	executedNodes         map[string]struct{}

	edgeIndex              domain.EdgeIndex
	executionCountByNodeID map[string]int

	mutex sync.Mutex

	userID *string // Optional, Only filled in testing workflows

	integrationSelector domain.IntegrationSelector
	enableEvents        bool
	enableStreaming     bool

	IsTestingWorkflow bool

	WorkflowExecutionStartedAt time.Time

	client   flowbaker.ClientInterface
	observer *executionObserver

	historyRecorder *HistoryRecorder
	usageCollector  *UsageCollector

	streamEventPublisher domain.StreamEventPublisher
}

type WorkflowExecutorDeps struct {
	ExecutionID           string
	UserID                *string
	Workflow              domain.Workflow
	Selector              domain.IntegrationSelector
	EnableEvents          bool
	EnableStreaming       bool
	IsTestingWorkflow     bool
	ExecutorClient        flowbaker.ClientInterface
	OrderedEventPublisher domain.EventPublisher
}

func NewWorkflowExecutor(deps WorkflowExecutorDeps) (WorkflowExecutor, error) {
	edgeIndex := domain.NewEdgeIndex(deps.Workflow)

	streamEventPublisher, err := domain.NewStreamEventPublisher(context.TODO(), deps.Workflow.WorkspaceID, deps.ExecutorClient)
	if err != nil {
		return WorkflowExecutor{}, fmt.Errorf("failed to create stream event publisher: %w", err)
	}

	streamEventPublisher.Initialize()

	observer := NewExecutionObserver()

	historyRecorder := NewHistoryRecorder()
	usageCollector := NewUsageCollector()
	eventBroadcaster := NewEventBroadcaster(
		deps.OrderedEventPublisher,
		deps.EnableEvents,
		deps.Workflow.ID,
		deps.ExecutionID,
	)
	streamBroadcaster := NewStreamEventBroadcaster(streamEventPublisher)

	observer.Subscribe(historyRecorder)
	observer.Subscribe(usageCollector)
	observer.Subscribe(eventBroadcaster)
	observer.SubscribeStream(streamBroadcaster)

	return WorkflowExecutor{
		executionID:                deps.ExecutionID,
		userID:                     deps.UserID,
		workflow:                   deps.Workflow,
		waitingExecutionTasks:      []WaitingExecutionTask{},
		executionQueue:             []NodeExecutionTask{},
		executedNodes:              map[string]struct{}{},
		edgeIndex:                  edgeIndex,
		integrationSelector:        deps.Selector,
		executionCountByNodeID:     map[string]int{},
		enableEvents:               deps.EnableEvents,
		enableStreaming:            deps.EnableStreaming,
		IsTestingWorkflow:          deps.IsTestingWorkflow,
		WorkflowExecutionStartedAt: time.Now(),
		client:                     deps.ExecutorClient,
		observer:                   observer,
		historyRecorder:            historyRecorder,
		usageCollector:             usageCollector,
		streamEventPublisher:       streamEventPublisher,
	}, nil
}

const (
	DefaultNodeExecutionLimit = 1000
)

func (w *WorkflowExecutor) Execute(ctx context.Context, nodeID string, items []domain.Item) (ExecutionResult, error) {
	workspaceID := w.workflow.WorkspaceID
	defer w.streamEventPublisher.Close()

	isErrorTrigger := w.IsErrorTrigger(nodeID)

	triggerNode, exists := w.workflow.GetNodeByID(nodeID)
	if !exists {
		return ExecutionResult{}, fmt.Errorf("node %s not found in workflow", nodeID)
	}

	inputPayload, err := json.Marshal(items)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("failed to marshal items: %w", err)
	}

	ctx = domain.NewContextWithEventOrder(ctx)
	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, domain.NewContextWithWorkflowExecutionContextParams{
		UserID:              w.userID,
		InputPayload:        inputPayload,
		WorkspaceID:         workspaceID,
		WorkflowID:          w.workflow.ID,
		WorkflowExecutionID: w.executionID,
		EnableEvents:        w.enableEvents,
		Observer:            w.observer,
		IsFromErrorTrigger:  isErrorTrigger,
		IsTesting:           w.IsTestingWorkflow,
		TriggerNode:         triggerNode,
	})

	if err := w.observer.Notify(ctx, WorkflowExecutionStartedEvent{
		Timestamp: time.Now(),
	}); err != nil {
		log.Error().Err(err).Msg("Failed to notify workflow execution started")
	}

	log.Info().Msgf("Executing workflow triggered by node %s", nodeID)

	// Queue trigger node as first execution task
	w.AddExecutionTask(NodeExecutionTask{
		NodeID: nodeID,
		ItemsByInputIndex: map[int]domain.NodeItems{
			0: {
				FromNodeID: nodeID,
				Items:      items,
			}},
	})

	executionCount := 0

	for len(w.executionQueue) > 0 {
		if ctx.Err() != nil {
			return ExecutionResult{}, ctx.Err()
		}

		execution := w.executionQueue[0]
		w.executionQueue = w.executionQueue[1:]

		executionCount++

		if w.IsExecutionLimitReached(execution.NodeID) {
			break
		}

		_, err := w.ExecuteNode(ctx, ExecuteNodeParams{
			Task:           execution,
			ExecutionOrder: int64(executionCount),
			Propagate:      true,
		})
		if err != nil {
			log.Error().Err(err).Msg("Error executing node")

			errNotify := w.observer.Notify(ctx, NodeExecutionFailedEvent{
				NodeID:            execution.NodeID,
				ItemsByInputIndex: execution.ItemsByInputIndex,
				Error:             err,
				Timestamp:         time.Now(),
			})
			if errNotify != nil {
				log.Error().Err(errNotify).Str("workflow_id", w.workflow.ID).Msg("executor: failed to notify node failed event")
			}

			break
		}

		w.FlushWaitingTasks()
	}

	executionResults := w.historyRecorder.GetHistoryEntries()

	nodeExecutions := mappers.DomainNodeExecutionsToFlowbaker(w.usageCollector.GetNodeExecutions())
	historyEntries := mappers.DomainNodeExecutionEntriesToFlowbaker(executionResults)

	completeParams := &flowbaker.CompleteExecutionRequest{
		ExecutionID:       w.executionID,
		WorkspaceID:       w.workflow.WorkspaceID,
		WorkflowID:        w.workflow.ID,
		TriggerNodeID:     nodeID,
		StartedAt:         w.WorkflowExecutionStartedAt,
		EndedAt:           time.Now(),
		NodeExecutions:    nodeExecutions,
		HistoryEntries:    historyEntries,
		IsTestingWorkflow: w.IsTestingWorkflow,
	}

	if err := w.client.CompleteWorkflowExecution(ctx, completeParams); err != nil {
		log.Error().Err(err).Msg("Failed to send complete workflow execution request")
	}

	if err := w.observer.Notify(ctx, WorkflowExecutionCompletedEvent{
		Timestamp: time.Now(),
	}); err != nil {
		log.Error().Err(err).Msg("Failed to notify workflow execution completed")
	}

	log.Info().Msg("Execution stack is empty, workflow is finished")

	executionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return ExecutionResult{}, errors.New("workflow execution context not found")
	}

	return ExecutionResult{
		Payload:              executionContext.ResponsePayload,
		Headers:              executionContext.ResponseHeaders,
		StatusCode:           executionContext.ResponseStatusCode,
		NodeExecutionResults: executionResults,
	}, nil
}

type ExecutionNodeType string

const (
	ExecutionNodeTypeAction  ExecutionNodeType = "action"
	ExecutionNodeTypeTrigger ExecutionNodeType = "trigger"
)

type ExecuteNodeParams struct {
	Task           NodeExecutionTask
	ExecutionOrder int64
	Propagate      bool
}

type ExecuteNodeResult struct {
	ItemsByOutputIndex map[int]domain.NodeItems
}

func (w *WorkflowExecutor) ExecuteNode(ctx context.Context, p ExecuteNodeParams) (ExecuteNodeResult, error) {
	execution := p.Task
	executionOrder := p.ExecutionOrder
	propagate := p.Propagate

	var result NodeExecutionResult
	var nodeID string

	node, exists := w.workflow.GetNodeByID(execution.NodeID)
	if !exists {
		return ExecuteNodeResult{}, fmt.Errorf("node %s not found in workflow", execution.NodeID)
	}

	nodeID = node.ID

	w.mutex.Lock()
	executionCount := w.executionCountByNodeID[nodeID]
	w.executionCountByNodeID[nodeID] = executionCount + 1
	w.mutex.Unlock()

	nodeExecutionStartedAt := time.Now()

	err := w.observer.Notify(ctx, NodeExecutionStartedEvent{
		NodeID:    execution.NodeID,
		Timestamp: nodeExecutionStartedAt,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to notify node execution started")
	}

	switch node.Type {
	case domain.NodeTypeAction:
		result, err = w.ExecuteActionNode(ctx, node, execution)
	case domain.NodeTypeTrigger:
		result, err = w.ExecuteTriggerNode(ctx, node, execution)
	default:
		return ExecuteNodeResult{}, fmt.Errorf("node type is invalid: %s", node.Type)
	}

	if err != nil {
		result, err = w.HandleNodeExecutionError(HandleNodeExecutionErrorParams{
			Err:  err,
			Node: node,
		})
		if err != nil {
			return ExecuteNodeResult{}, err
		}
	}

	nodeExecutionEndedAt := time.Now()

	if propagate {
		for outputIndex, nodeItems := range result.Output.ItemsByOutputIndex {
			nodes := w.edgeIndex.GetTargetNodes(nodeID, outputIndex)
			if len(nodes) == 0 {
				continue
			}

			if len(nodeItems.Items) > 0 {
				for _, node := range nodes {
					err := w.AddTaskForDownstreamNode(ctx, AddTaskForDownstreamNodeParams{
						FromNodeID:  nodeID,
						Node:        node,
						Items:       nodeItems.Items,
						OutputIndex: outputIndex,
					})
					if err != nil {
						return ExecuteNodeResult{}, err
					}
				}
			}
		}
	}

	w.MarkNodeAsExecuted(nodeID)

	itemsByOutputIndex := map[int]domain.NodeItems{}
	for idx, nodeItems := range result.Output.ItemsByOutputIndex {
		itemsByOutputIndex[idx] = nodeItems
	}

	err = w.observer.Notify(ctx, NodeExecutionCompletedEvent{
		NodeID:                nodeID,
		ItemsByInputIndex:     execution.ItemsByInputIndex,
		ItemsByOutputIndex:    itemsByOutputIndex,
		ExecutionOrder:        executionOrder,
		IntegrationType:       result.IntegrationType,
		IntegrationActionType: result.IntegrationActionType,
		StartedAt:             nodeExecutionStartedAt,
		EndedAt:               nodeExecutionEndedAt,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to notify node execution completed")
	}

	return ExecuteNodeResult{
		ItemsByOutputIndex: itemsByOutputIndex,
	}, nil
}

func (w *WorkflowExecutor) ExecuteTriggerNode(ctx context.Context, node domain.WorkflowNode, execution NodeExecutionTask) (NodeExecutionResult, error) {
	inputPayload, exists := execution.ItemsByInputIndex[0]
	if !exists {
		return NodeExecutionResult{}, fmt.Errorf("trigger input items not found")
	}

	return NodeExecutionResult{
		Output: domain.IntegrationOutput{
			ItemsByOutputIndex: []domain.NodeItems{inputPayload},
		},
		IntegrationType:       domain.IntegrationType(node.Type),
		IntegrationActionType: domain.IntegrationActionType(node.TriggerNodeOpts.EventType),
	}, nil
}

func (w *WorkflowExecutor) ExecuteActionNode(ctx context.Context, node domain.WorkflowNode, execution NodeExecutionTask) (NodeExecutionResult, error) {
	if w.ShouldSkip(node) {
		return NodeExecutionResult{
			Output: domain.IntegrationOutput{
				ItemsByOutputIndex: []domain.NodeItems{},
			},
			IntegrationType:       domain.IntegrationType(node.IntegrationType),
			IntegrationActionType: node.ActionNodeOpts.ActionType,
		}, nil
	}

	integrationCreator, err := w.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
		IntegrationType: node.IntegrationType,
	})
	if err != nil {
		return NodeExecutionResult{}, err
	}

	credentialID, exists := node.IntegrationSettings["credential_id"]
	if !exists {
		credentialID = ""
	}

	credentialIDString, ok := credentialID.(string)
	if !ok {
		return NodeExecutionResult{}, fmt.Errorf("credential_id is not a string")
	}

	integrationExecutor, err := integrationCreator.CreateIntegration(ctx, domain.CreateIntegrationParams{
		WorkspaceID:  w.workflow.WorkspaceID,
		CredentialID: credentialIDString,
	})
	if err != nil {
		return NodeExecutionResult{}, err
	}

	output, err := integrationExecutor.Execute(ctx, domain.IntegrationInput{
		NodeID:            node.ID,
		ItemsByInputIndex: execution.ItemsByInputIndex,
		Workflow:          &w.workflow,
		IntegrationParams: domain.IntegrationParams{
			Settings: node.IntegrationSettings,
		},
		ActionType: node.ActionNodeOpts.ActionType,
	})
	if err != nil {

		return NodeExecutionResult{}, err
	}

	return NodeExecutionResult{
		Output:                output,
		IntegrationType:       domain.IntegrationType(node.IntegrationType),
		IntegrationActionType: node.ActionNodeOpts.ActionType,
	}, nil
}

type AddTaskForDownstreamNodeParams struct {
	FromNodeID  string
	Node        domain.WorkflowNode
	OutputIndex int
	Items       []domain.Item
}

func (w *WorkflowExecutor) AddTaskForDownstreamNode(ctx context.Context, p AddTaskForDownstreamNodeParams) error {
	node := p.Node
	items := p.Items
	outputIndex := p.OutputIndex

	edge, found := w.workflow.FindEdge(node.ID, p.FromNodeID, outputIndex)
	if !found {
		return fmt.Errorf("input index not found for subscribed output index %d", outputIndex)
	}

	matchingInputIndex := edge.TargetIndex

	if w.ShouldWait(node) {
		w.HandleWaitingTask(HandleWaitingTaskParams{
			FromNodeID: p.FromNodeID,
			Node:       node,
			InputIndex: matchingInputIndex,
			Items:      items,
		})

		return nil
	}

	t := NodeExecutionTask{
		NodeID: node.ID,
		ItemsByInputIndex: map[int]domain.NodeItems{
			matchingInputIndex: {
				FromNodeID: p.FromNodeID,
				Items:      items,
			},
		},
	}

	w.AddExecutionTask(t)

	return nil
}

func (w *WorkflowExecutor) ShouldWait(node domain.WorkflowNode) bool {
	inputIndices := w.workflow.GetConnectedInputIndices(node.ID)

	excludedTypes := []domain.IntegrationType{
		domain.IntegrationType_AIAgent,
	}

	return len(inputIndices) > 1 && !slices.Contains(excludedTypes, node.IntegrationType)
}

func (w *WorkflowExecutor) ShouldSkip(node domain.WorkflowNode) bool {
	return node.UsageContext != "" && node.UsageContext != string(domain.UsageContextWorkflow)
}

type HandleWaitingTaskParams struct {
	FromNodeID string
	Node       domain.WorkflowNode
	InputIndex int
	Items      []domain.Item
}

func (w *WorkflowExecutor) HandleWaitingTask(p HandleWaitingTaskParams) {
	waitingTask, exists := w.GetAvailableWaitingTask(p.Node.ID, p.InputIndex)

	if !exists {
		t := NewWaitingExecutionTask(p.Node.ID, map[int]domain.NodeItems{
			p.InputIndex: {
				FromNodeID: p.FromNodeID,
				Items:      p.Items,
			},
		})

		w.AddWaitingExecutionTask(t)
		return
	}

	waitingTask.AddItems(p.FromNodeID, p.InputIndex, p.Items)

	if w.shouldExecuteWaitingTask(waitingTask, p.Node) {
		w.ResolveWaitingTask(p.Node.ID, waitingTask)
	}
}

func (w *WorkflowExecutor) shouldExecuteWaitingTask(waitingTask WaitingExecutionTask, node domain.WorkflowNode) bool {
	waitingTask.mutex.Lock()
	defer waitingTask.mutex.Unlock()

	inputIndices := w.workflow.GetConnectedInputIndices(node.ID)

	for _, inputIndex := range inputIndices {
		if _, exists := waitingTask.ReceivedPayloads[inputIndex]; !exists {
			return false
		}
	}

	return true
}

func (w *WorkflowExecutor) GetAvailableWaitingTask(nodeID string, inputIndex int) (WaitingExecutionTask, bool) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	for _, task := range w.waitingExecutionTasks {
		if task.NodeID == nodeID {
			if _, filled := task.ReceivedPayloads[inputIndex]; !filled {
				return task, true
			}
		}
	}

	return WaitingExecutionTask{}, false
}

func (w *WorkflowExecutor) FlushWaitingTasks() {
	if len(w.executionQueue) > 0 || len(w.waitingExecutionTasks) == 0 {
		return
	}

	for _, task := range w.waitingExecutionTasks {
		w.AddExecutionTask(NodeExecutionTask{
			NodeID:            task.NodeID,
			ItemsByInputIndex: task.ReceivedPayloads,
		})
	}

	w.waitingExecutionTasks = []WaitingExecutionTask{}
}

func (w *WorkflowExecutor) AddExecutionTask(execution NodeExecutionTask) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	log.Info().Msgf("Adding execution task for node %s", execution.NodeID)

	unshift := true

	if unshift {
		w.executionQueue = append([]NodeExecutionTask{execution}, w.executionQueue...)
		return
	}

	w.executionQueue = append(w.executionQueue, execution)
}

func (w *WorkflowExecutor) MarkNodeAsExecuted(nodeID string) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.executedNodes[nodeID] = struct{}{}
}

func (w *WorkflowExecutor) IsNodeExecuted(nodeID string) bool {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	_, exists := w.executedNodes[nodeID]

	return exists
}

func (w *WorkflowExecutor) AddWaitingExecutionTask(task WaitingExecutionTask) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	log.Info().Msgf("Adding waiting task %s for node %s", task.ID, task.NodeID)

	w.waitingExecutionTasks = append(w.waitingExecutionTasks, task)
}

func (w *WorkflowExecutor) RemoveWaitingExecutionTask(taskID string) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	filtered := []WaitingExecutionTask{}

	for _, task := range w.waitingExecutionTasks {
		if task.ID != taskID {
			filtered = append(filtered, task)
		}
	}

	w.waitingExecutionTasks = filtered
}

func (w *WorkflowExecutor) ResolveWaitingTask(nodeID string, task WaitingExecutionTask) {
	w.AddExecutionTask(NodeExecutionTask{
		NodeID:            nodeID,
		ItemsByInputIndex: task.ReceivedPayloads,
	})

	w.RemoveWaitingExecutionTask(task.ID)
}

type ErrorItem struct {
	ErrorMessage string `json:"error_message"`
}

type HandleNodeExecutionErrorParams struct {
	Err  error
	Node domain.WorkflowNode
}

func (w *WorkflowExecutor) HandleNodeExecutionError(p HandleNodeExecutionErrorParams) (NodeExecutionResult, error) {
	settings := p.Node.Settings

	if !settings.ReturnErrorAsItem {
		return NodeExecutionResult{}, p.Err
	}

	errorItem := ErrorItem{
		ErrorMessage: p.Err.Error(),
	}

	integrationType := domain.IntegrationType(p.Node.Type)

	actionType := ""

	switch p.Node.Type {
	case domain.NodeTypeAction:
		actionType = string(p.Node.ActionNodeOpts.ActionType)
	case domain.NodeTypeTrigger:
		actionType = string(p.Node.TriggerNodeOpts.EventType)
	}

	return NodeExecutionResult{
		Output: domain.IntegrationOutput{
			ItemsByOutputIndex: []domain.NodeItems{
				{Items: []domain.Item{errorItem}},
			},
		},
		IntegrationType:       integrationType,
		IntegrationActionType: domain.IntegrationActionType(actionType),
	}, nil
}

func (w *WorkflowExecutor) IsErrorTrigger(nodeID string) bool {
	node, exists := w.workflow.GetNodeByID(nodeID)
	if !exists {
		return false
	}

	return node.Type == domain.NodeTypeTrigger && node.TriggerNodeOpts.EventType == "on_error"
}

func (w *WorkflowExecutor) IsExecutionLimitReached(nodeID string) bool {
	node, exists := w.workflow.GetNodeByID(nodeID)
	if !exists {
		return false
	}

	count, exists := w.executionCountByNodeID[nodeID]
	if !exists {
		return false
	}

	limit := DefaultNodeExecutionLimit

	if node.Settings.OverwriteExecutionLimit && node.Settings.ExecutionLimit > 0 {
		limit = node.Settings.ExecutionLimit
	}

	if w.workflow.Settings.NodeExecutionLimit > 0 {
		limit = w.workflow.Settings.NodeExecutionLimit
	}

	isLimitReached := count >= limit

	if isLimitReached {
		log.Error().Msgf("node %s execution limit reached (limit: %d, count: %d)", nodeID, limit, count)
	}

	return isLimitReached
}

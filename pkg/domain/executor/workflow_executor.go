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

	pauseResult *pauseResult
	resumeState *domain.ResumeState
}

type pauseResult struct {
	NodeID         string
	WakeAt         time.Time
	SleepNodeInput domain.NodeItemsMap
}

func (w *WorkflowExecutor) buildResumeStateSnapshot(ctx context.Context, triggerNodeID string) *domain.ResumeState {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	waitingTasks := make([]domain.WaitingTaskSnapshot, 0, len(w.waitingExecutionTasks))
	for _, t := range w.waitingExecutionTasks {
		waitingTasks = append(waitingTasks, domain.WaitingTaskSnapshot{
			NodeID:           t.NodeID,
			ReceivedPayloads: t.ReceivedPayloads,
		})
	}

	queuedTasks := make([]domain.QueuedTaskSnapshot, 0, len(w.executionQueue))
	for _, t := range w.executionQueue {
		queuedTasks = append(queuedTasks, domain.QueuedTaskSnapshot{
			NodeID:            t.NodeID,
			ItemsByInputIndex: t.ItemsByInputIndex,
		})
	}

	executedNodes := make([]string, 0, len(w.executedNodes))
	for nodeID := range w.executedNodes {
		executedNodes = append(executedNodes, nodeID)
	}

	executionCount := make(map[string]int, len(w.executionCountByNodeID))
	for nodeID, count := range w.executionCountByNodeID {
		executionCount[nodeID] = count
	}

	lastEventOrder := 0
	if orderCtx, ok := domain.GetEventOrderContext(ctx); ok {
		lastEventOrder = orderCtx.GetCurrentOrder()
	}

	return &domain.ResumeState{
		SleepNodeID:            w.pauseResult.NodeID,
		TriggerNodeID:          triggerNodeID,
		SleepNodeInput:         w.pauseResult.SleepNodeInput,
		WaitingTasks:           waitingTasks,
		QueuedTasks:            queuedTasks,
		ExecutedNodes:          executedNodes,
		ExecutionCountByNodeID: executionCount,
		LastEventOrder:         lastEventOrder,
	}
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
	ResumeState           *domain.ResumeState
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

	waitingTasks := []WaitingExecutionTask{}
	queuedTasks := []NodeExecutionTask{}
	executedNodes := map[string]struct{}{}
	executionCountByNodeID := map[string]int{}

	if deps.ResumeState != nil {
		for _, snap := range deps.ResumeState.WaitingTasks {
			waitingTasks = append(waitingTasks, NewWaitingExecutionTask(snap.NodeID, snap.ReceivedPayloads))
		}
		for _, snap := range deps.ResumeState.QueuedTasks {
			queuedTasks = append(queuedTasks, NodeExecutionTask{
				NodeID:            snap.NodeID,
				ItemsByInputIndex: snap.ItemsByInputIndex,
			})
		}
		for _, nodeID := range deps.ResumeState.ExecutedNodes {
			executedNodes[nodeID] = struct{}{}
		}
		for nodeID, count := range deps.ResumeState.ExecutionCountByNodeID {
			executionCountByNodeID[nodeID] = count
		}
	}

	return WorkflowExecutor{
		executionID:                deps.ExecutionID,
		userID:                     deps.UserID,
		workflow:                   deps.Workflow,
		waitingExecutionTasks:      waitingTasks,
		executionQueue:             queuedTasks,
		executedNodes:              executedNodes,
		edgeIndex:                  edgeIndex,
		integrationSelector:        deps.Selector,
		executionCountByNodeID:     executionCountByNodeID,
		enableEvents:               deps.EnableEvents,
		enableStreaming:            deps.EnableStreaming,
		IsTestingWorkflow:          deps.IsTestingWorkflow,
		WorkflowExecutionStartedAt: time.Now(),
		client:                     deps.ExecutorClient,
		observer:                   observer,
		historyRecorder:            historyRecorder,
		usageCollector:             usageCollector,
		streamEventPublisher:       streamEventPublisher,
		resumeState:                deps.ResumeState,
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

	startOrder := 0
	if w.resumeState != nil {
		startOrder = w.resumeState.LastEventOrder
	}
	ctx = domain.NewContextWithEventOrderFrom(ctx, startOrder)
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

	isResume := w.resumeState != nil

	if !isResume {
		if err := w.observer.Notify(ctx, WorkflowExecutionStartedEvent{
			Timestamp: time.Now(),
		}); err != nil {
			log.Error().Err(err).Msg("Failed to notify workflow execution started")
		}
	}

	log.Info().Bool("resume", isResume).Msgf("Executing workflow triggered by node %s", nodeID)

	executionCount := 0

	if isResume {
		sleepNode, exists := w.workflow.GetNodeByID(w.resumeState.SleepNodeID)
		if !exists {
			return ExecutionResult{}, fmt.Errorf("resume: sleep node %s not found in workflow", w.resumeState.SleepNodeID)
		}

		executionCount++

		sleepOutput := domain.IntegrationOutput{
			ItemsByOutputIndex: w.resumeState.SleepNodeInput,
		}

		if err := w.Propagate(ctx, w.resumeState.SleepNodeID, sleepOutput); err != nil {
			return ExecutionResult{}, fmt.Errorf("resume: failed to propagate sleep output: %w", err)
		}

		w.MarkNodeAsExecuted(w.resumeState.SleepNodeID)

		now := time.Now()
		if err := w.observer.Notify(ctx, NodeExecutionCompletedEvent{
			NodeID:                w.resumeState.SleepNodeID,
			ItemsByInputIndex:     w.resumeState.SleepNodeInput,
			ItemsByOutputIndex:    sleepOutput.ItemsByOutputIndex,
			ExecutionOrder:        int64(executionCount),
			IntegrationType:       sleepNode.IntegrationType,
			IntegrationActionType: sleepNode.ActionNodeOpts.ActionType,
			StartedAt:             now,
			EndedAt:               now,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to notify sleep node completion on resume")
		}
	} else {
		w.AddExecutionTask(NodeExecutionTask{
			NodeID:            nodeID,
			ItemsByInputIndex: domain.NewNodeItemsMap(0, nodeID, items),
		})
	}

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

		if w.pauseResult != nil {
			break
		}

		w.FlushWaitingTasks()
	}

	executionResults := w.historyRecorder.GetHistoryEntries()

	nodeExecutions := mappers.DomainNodeExecutionsToFlowbaker(w.usageCollector.GetNodeExecutions())
	historyEntries := mappers.DomainNodeExecutionEntriesToFlowbaker(executionResults)

	if w.pauseResult != nil {
		originalTriggerID := nodeID
		if w.resumeState != nil && w.resumeState.TriggerNodeID != "" {
			originalTriggerID = w.resumeState.TriggerNodeID
		}

		sleepEntry := domain.NodeExecutionEntry{
			NodeID:             w.pauseResult.NodeID,
			ItemsByInputIndex:  w.pauseResult.SleepNodeInput,
			ItemsByOutputIndex: domain.NodeItemsMap{},
			EventType:          domain.NodeExecutionStarted,
			Timestamp:          time.Now().UnixNano(),
		}
		executionResults = append(executionResults, sleepEntry)
		historyEntries = mappers.DomainNodeExecutionEntriesToFlowbaker(executionResults)

		if err := w.observer.Notify(ctx, WorkflowExecutionPausedEvent{
			SleepNodeID: w.pauseResult.NodeID,
			WakeAt:      w.pauseResult.WakeAt,
			Timestamp:   time.Now(),
		}); err != nil {
			log.Error().Err(err).Msg("Failed to notify workflow execution paused")
		}

		snapshot := w.buildResumeStateSnapshot(ctx, originalTriggerID)
		snapshotJSON, err := json.Marshal(snapshot)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("failed to marshal resume state: %w", err)
		}

		pauseUserID := ""
		if w.userID != nil {
			pauseUserID = *w.userID
		}
		pauseParams := &flowbaker.PauseExecutionRequest{
			ExecutionID:       w.executionID,
			WorkspaceID:       w.workflow.WorkspaceID,
			WorkflowID:        w.workflow.ID,
			UserID:            pauseUserID,
			SleepNodeID:       w.pauseResult.NodeID,
			WakeAt:            w.pauseResult.WakeAt,
			StartedAt:         w.WorkflowExecutionStartedAt,
			PausedAt:          time.Now(),
			NodeExecutions:    nodeExecutions,
			HistoryEntries:    historyEntries,
			IsTestingWorkflow: w.IsTestingWorkflow,
			ResumeStateJSON:   snapshotJSON,
		}

		if err := w.client.PauseWorkflowExecution(ctx, pauseParams); err != nil {
			return ExecutionResult{}, fmt.Errorf("failed to send pause: %w", err)
		}

		log.Info().Str("sleep_node_id", w.pauseResult.NodeID).Time("wake_at", w.pauseResult.WakeAt).Msg("Workflow paused")

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

	if isResume {
		completeParams.ResumedFromSleep = true
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
	ItemsByOutputIndex domain.NodeItemsMap
}

func (w *WorkflowExecutor) ExecuteNode(ctx context.Context, p ExecuteNodeParams) (ExecuteNodeResult, error) {
	task := p.Task

	node, exists := w.workflow.GetNodeByID(task.NodeID)
	if !exists {
		return ExecuteNodeResult{}, fmt.Errorf("node %s not found in workflow", task.NodeID)
	}

	w.mutex.Lock()
	executionCount := w.executionCountByNodeID[node.ID]
	w.executionCountByNodeID[node.ID] = executionCount + 1
	w.mutex.Unlock()

	nodeExecutionStartedAt := time.Now()

	if err := w.observer.Notify(ctx, NodeExecutionStartedEvent{
		NodeID:    task.NodeID,
		Timestamp: nodeExecutionStartedAt,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to notify node execution started")
	}

	var result NodeExecutionResult
	var err error

	switch node.Type {
	case domain.NodeTypeAction:
		result, err = w.ExecuteActionNode(ctx, node, task)
	case domain.NodeTypeTrigger:
		result, err = w.ExecuteTriggerNode(ctx, node, task)
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

	if execCtx, ok := domain.GetWorkflowExecutionContext(ctx); ok {
		for _, sig := range execCtx.DrainSignals() {
			switch s := sig.(type) {
			case domain.PauseSignal:
				w.pauseResult = &pauseResult{
					NodeID:         node.ID,
					WakeAt:         s.WakeAt,
					SleepNodeInput: result.Output.ItemsByOutputIndex,
				}
				return ExecuteNodeResult{}, nil
			}
		}
	}

	nodeExecutionEndedAt := time.Now()

	if p.Propagate {
		err := w.Propagate(ctx, node.ID, result.Output)
		if err != nil {
			return ExecuteNodeResult{}, err
		}
	}

	w.MarkNodeAsExecuted(node.ID)

	err = w.observer.Notify(ctx, NodeExecutionCompletedEvent{
		NodeID:                node.ID,
		ItemsByInputIndex:     task.ItemsByInputIndex,
		ItemsByOutputIndex:    result.Output.ItemsByOutputIndex,
		ExecutionOrder:        p.ExecutionOrder,
		IntegrationType:       result.IntegrationType,
		IntegrationActionType: result.IntegrationActionType,
		StartedAt:             nodeExecutionStartedAt,
		EndedAt:               nodeExecutionEndedAt,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to notify node execution completed")
	}

	return ExecuteNodeResult{
		ItemsByOutputIndex: result.Output.ItemsByOutputIndex,
	}, nil
}

func (w *WorkflowExecutor) ExecuteTriggerNode(ctx context.Context, node domain.WorkflowNode, execution NodeExecutionTask) (NodeExecutionResult, error) {
	inputPayload, exists := execution.ItemsByInputIndex[0]
	if !exists {
		return NodeExecutionResult{}, fmt.Errorf("trigger input items not found")
	}

	return NodeExecutionResult{
		Output: domain.IntegrationOutput{
			ItemsByOutputIndex: domain.NewNodeItemsMap(0, inputPayload.FromNodeID, inputPayload.Items),
		},
		IntegrationType:       domain.IntegrationType(node.Type),
		IntegrationActionType: domain.IntegrationActionType(node.TriggerNodeOpts.EventType),
	}, nil
}

func (w *WorkflowExecutor) ExecuteActionNode(ctx context.Context, node domain.WorkflowNode, execution NodeExecutionTask) (NodeExecutionResult, error) {
	if w.ShouldSkip(node) {
		return NodeExecutionResult{
			Output: domain.IntegrationOutput{
				ItemsByOutputIndex: domain.NodeItemsMap{},
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

func (w *WorkflowExecutor) Propagate(ctx context.Context, nodeID string, output domain.IntegrationOutput) error {
	for outputIndex, nodeItems := range output.ItemsByOutputIndex {
		if len(nodeItems.Items) == 0 {
			continue
		}

		nodes := w.edgeIndex.GetTargetNodes(nodeID, outputIndex)

		for _, node := range nodes {
			err := w.AddTaskForDownstreamNode(ctx, AddTaskForDownstreamNodeParams{
				FromNodeID:  nodeID,
				Node:        node,
				Items:       nodeItems.Items,
				OutputIndex: outputIndex,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
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

	w.AddExecutionTask(NodeExecutionTask{
		NodeID:            node.ID,
		ItemsByInputIndex: domain.NewNodeItemsMap(matchingInputIndex, p.FromNodeID, items),
	})

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
		newTask := NewWaitingExecutionTask(
			p.Node.ID,
			domain.NewNodeItemsMap(p.InputIndex, p.FromNodeID, p.Items),
		)

		w.AddWaitingExecutionTask(newTask)

		return
	}

	waitingTask.AddItems(p.FromNodeID, p.InputIndex, p.Items)

	if w.ShouldResolveWaitingTask(waitingTask, p.Node) {
		w.ResolveWaitingTask(p.Node.ID, waitingTask)
	}
}

func (w *WorkflowExecutor) ShouldResolveWaitingTask(waitingTask WaitingExecutionTask, node domain.WorkflowNode) bool {
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
		w.AddExecutionTask(task.ToExecutionTask())
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

type HandleNodeExecutionErrorParams struct {
	Err  error
	Node domain.WorkflowNode
}

func (w *WorkflowExecutor) HandleNodeExecutionError(p HandleNodeExecutionErrorParams) (NodeExecutionResult, error) {
	settings := p.Node.Settings

	if !settings.ReturnErrorAsItem {
		return NodeExecutionResult{}, p.Err
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
		Output:                domain.NewErrorIntegrationOutput(p.Err),
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
